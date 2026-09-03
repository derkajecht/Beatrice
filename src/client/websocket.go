package client

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/derkajecht/Beatrice/src/client/tui"
	"github.com/derkajecht/Beatrice/src/shared"
)

func (u *User) ConnectWithRetry(ctx context.Context, retryCount int) (*websocket.Conn, error) {
	var lastErr error
	for i := range retryCount {
		conn, _, err := websocket.Dial(ctx, u.Addr, nil)
		if err == nil {
			return conn, nil
		}

		// log the error and increment the retry count
		slog.Error("Error connecting to server", "err", err, "retry", i+1)
		lastErr = err

		// set delay and jitter for exponential backoff
		delay := time.Duration(1<<uint(i)) * time.Second
		jitter := time.Duration(rand.Intn(1000)) * time.Millisecond
		select {
		case <-time.After(delay + jitter):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return nil, fmt.Errorf("failed to connect after %d retries: %w", retryCount, lastErr)
}

// NewChatClient establishes a connection to the server using the host and port provided.
func (u *User) NewChatClient(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			slog.Info("Shutting down connection loop")
			return

		default:
			func() {
				dialCtx, dialCancel := context.WithTimeout(ctx, 30*time.Second)
				defer dialCancel()

				// ctx.Done() called inside ConnectWithRetry
				conn, err := u.ConnectWithRetry(dialCtx, 3) // retry 3 times
				if err != nil {
					slog.Error("Error connecting to server", "err", err)
					return
				}

				// create a new client instance, pass the connection and a channel for tui messages
				u.mu.Lock()
				u.Conn = conn
				u.mu.Unlock()
				defer conn.Close(websocket.StatusInternalError, "Client closed")

				conn.SetReadLimit(1 << 20) // 1 MiB

				// send handshake to server: identity key for auth/TOFU plus
				// the HPKE public key users will use to encrypt messages to us
				err = u.SendPacketToServer("h", shared.HandshakePacket{
					Nickname:   u.Nickname,
					PubKey:     u.Crypto.IdentityPubKey,
					HPKEPubKey: u.Crypto.PubKey,
				})
				if err != nil {
					slog.Error("failed to send handshake", "err", err)
				}

				// run readloop in a goroutine for async reading
				// uses parent ctx - signal.NotifyContext() - only cancels on system signals (ctrl + c to exit)
				u.readLoop(ctx) // INFO: need to put inside go routine?
			}()

			// inter cycle sleep to avoid retry immediately
			select {
			case <-ctx.Done():
				slog.Info("Shutting down readLoop")
				return
			default:
				time.Sleep(5 * time.Second)
			}
		}
	}
}

func (u *User) readLoop(ctx context.Context) {
	for {
		u.mu.RLock()
		conn := u.Conn
		u.mu.RUnlock()
		_, msg, err := conn.Read(ctx) // msg is []byte
		if err != nil {
			slog.Error("Error reading from websocket connection", "err", err)
			return
		}

		// unmarshal the message to json
		var packet shared.GeneralPacket
		err = json.Unmarshal(msg, &packet)
		if err != nil {
			slog.Error("Error unmarshalling message packet", "err", err)
			continue
		}
		slog.Debug("packet successfully unmarshalled", "packet", packet)

		if packet.Type == "n" {
			var np shared.NicknameUpdatePacket
			if err := json.Unmarshal(packet.Message, &np); err != nil {
				slog.Error("failed to unmarshal NicknameUpdatePacket", "err", err)
				continue
			}
			// update client side nickname with the
			u.Nickname = np.Nickname
		}

		// Handle challenge-response auth: sign the nonce with our identity key.
		if packet.Type == "c" {
			var cp shared.ChallengePacket
			if err := json.Unmarshal(packet.Message, &cp); err != nil {
				slog.Error("failed to unmarshal ChallengePacket", "err", err)
				continue
			}
			// sign the nonce with our identity key
			sig := ed25519.Sign(u.Crypto.IdentityPrivKey, []byte(cp.PendingAuth))
			// create response packet with the signature
			// send original nonce and signature back to server
			resp := shared.ChallengeResponse{Signature: sig, Nonce: cp.PendingAuth}
			if err := u.SendPacketToServer("c", resp); err != nil {
				slog.Error("failed to send challenge response", "err", err)
			}
			continue // challenge handled here, don't forward to the TUI
		}

		// Keep the local connected-users directory in sync before anything
		// is forwarded to the TUI, so outbound fan-out can encrypt per user.
		// Updates come from server-authoritative d/j/l packets only; the
		// key fields carry HPKE public keys.
		switch packet.Type {
		case "d":
			var dp shared.DirPacket
			if err := json.Unmarshal(packet.Message, &dp); err != nil {
				slog.Error("failed to unmarshal DirPacket", "err", err)
				continue
			}
			u.ApplyDirPacket(dp.CurrentUsers)
		case "j":
			var jp shared.JoinPacket
			if err := json.Unmarshal(packet.Message, &jp); err != nil {
				slog.Error("failed to unmarshal JoinPacket", "err", err)
				continue
			}
			u.ApplyJoin(jp.Nickname, jp.PubKey)
		case "l":
			var lp shared.LeavePacket
			if err := json.Unmarshal(packet.Message, &lp); err != nil {
				slog.Error("failed to unmarshal LeavePacket", "err", err)
				continue
			}
			u.ApplyLeave(lp.Nickname)
		case "m":
			// Receive-side decryption: open the HPKE payload locally and
			// forward a plaintext DecryptedMessage to the TUI. Failures are
			// dropped and logged; the server never sees or handles plaintext.
			var mp shared.MessagePacket
			if err := json.Unmarshal(packet.Message, &mp); err != nil {
				slog.Error("failed to unmarshal MessagePacket", "err", err)
				continue
			}
			plaintext, err := DecryptMessage(u, mp.Enc, mp.CT, mp.Sender, mp.Recipient)
			if err != nil {
				slog.Error("dropping undecryptable message", "sender", mp.Sender, "recipient", mp.Recipient, "err", err)
				continue
			}
			local, err := json.Marshal(shared.DecryptedMessage{Sender: mp.Sender, Content: plaintext, Time: mp.Time})
			if err != nil {
				slog.Error("failed to marshal decrypted message", "err", err)
				continue
			}
			packet.Message = local
		case "p":
			// Presence updates carry no secrets; validate the shape so
			// malformed packets are dropped before reaching the TUI.
			var pp shared.PresencePacket
			if err := json.Unmarshal(packet.Message, &pp); err != nil {
				slog.Error("failed to unmarshal PresencePacket", "err", err)
				continue
			}
		}

		select {
		case <-ctx.Done():
			slog.Info("Shutting down read loop")
			return

		case u.TuiChan <- packet: // send packet to the hub
		default:
			slog.Info("TuiChan buffer full, dropping packet")
			continue
		}
	}
}

// SendPacketToServer sends a packet or message to the client
// and returns an error if any
func (u *User) SendPacketToServer(packetType string, innerPacket any) error {
	u.mu.RLock()
	conn := u.Conn
	u.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("err_conn_is_nil")
	}

	// marshal inner packet to JSON
	innerBytes, err := json.Marshal(innerPacket)
	if err != nil {
		return err
	}

	// create envelope struct and wrap inner packet in it
	envelope := shared.GeneralPacket{
		Type:    packetType,
		Message: innerBytes,
	}

	writeCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return wsjson.Write(writeCtx, conn, envelope)
}

// BroadcastMessage encrypts content for every currently connected user
// (excluding self) and sends one MessagePacket per recipient. With no other
// users connected it returns an error so the TUI can surface it. Partial
// encryption failures still deliver to reachable recipients; an error is only
// returned when nothing could be encrypted or no send succeeded.
func (u *User) BroadcastMessage(content string) error {
	users := u.GetConnectedUserInfo()
	targets := make([]string, 0, len(users))
	for nickname := range users {
		if nickname == u.Nickname {
			continue
		}
		targets = append(targets, nickname)
	}
	if len(targets) == 0 {
		return fmt.Errorf("no other users connected")
	}

	encrypted, encErr := EncryptMessage(u, targets, content)
	if len(encrypted) == 0 {
		if encErr == nil {
			encErr = fmt.Errorf("no recipients could be encrypted")
		}
		return encErr
	}
	if encErr != nil {
		slog.Warn("partial encryption failure, sending to reachable recipients only", "err", encErr)
	}

	sentOK := 0
	var firstErr error
	for _, t := range encrypted {
		wire := shared.MessagePacket{
			Recipient: t.Nickname,
			Sender:    u.Nickname,
			Enc:       t.Enc,
			CT:        t.CT,
			Time:      time.Now(),
		}
		if err := u.SendPacketToServer("m", wire); err != nil {
			slog.Error("failed to send message", "recipient", t.Nickname, "err", err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		sentOK++
	}

	if sentOK == 0 && firstErr != nil {
		return firstErr
	}
	return nil
}

// SendPresence sends a presence packet to the server. This is a separate
// path from encrypted chat sending: presence is plaintext metadata the
// server is meant to see, validate, and fan out to peers. The server binds
// the nickname to the authenticated connection, so the wire nickname here is
// informational only.
func (u *User) SendPresence(status string) error {
	return u.SendPacketToServer("p", shared.PresencePacket{Nickname: u.Nickname, Status: status})
}

// StartClient establishes a connection to the server using the host and port
// provided and runs the TUI with the given configuration.
// It returns an error if the host or port is empty.
func StartClient(host, port, nickname, ephemeral string, cfg Config) error {
	// check if host or port is empty, default to localhost:8080
	if shared.HasEmptyArgs(host, port) {
		slog.Warn("No host or port provided: Defaulting to 'localhost:8080'")
		host = "localhost"
		port = "8080"
	}

	// Call crypto suite to generate a new key pair
	// stores the public and private keys in the user session
	ephemeralBool := ephemeral != ""
	cryptoPkt, err := NewUserSession(ephemeralBool)
	if err != nil {
		slog.Error("Error generating user session", "err", err)
		return err
	}

	// format the address string
	addr := fmt.Sprintf("ws://%s:%s/ws", host, port)

	// create a new context with a timeout of 30 seconds
	rootCtx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// create a new user instance
	// initiates the conn and tuichan fields
	user := NewUser()
	user.Addr = addr
	user.Nickname = nickname
	user.Crypto = *cryptoPkt
	go user.NewChatClient(rootCtx) // non-blocking

	logCh := LoggerSetup()
	send := func(content string) error {
		return user.BroadcastMessage(content)
	}
	sendPresence := func(status string) error {
		return user.SendPresence(status)
	}
	tui.ApplyTheme(cfg.Theme)
	program := tea.NewProgram(tui.NewModel(user.TuiChan, logCh, user.Nickname, send, sendPresence, cfg.InactivityTimeout), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := program.Run(); err != nil {
		return fmt.Errorf("tui error: %w", err)
	}
	return nil
}
