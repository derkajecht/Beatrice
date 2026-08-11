package client

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/derkajecht/Beatrice/internal/shared"
)

func (u *User) ConnectWithRetry(ctx context.Context, retryCount int) (*websocket.Conn, error) {
	var lastErr error
	for i := range retryCount {
		conn, _, err := websocket.Dial(ctx, u.Addr, nil)
		if err == nil {
			return conn, nil
		}

		// 	log the error and increment the retry count
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
		slog.Info("packet successfully unmarshalled", "packet", packet)

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

// SendPacketToClient sends a packet or message to the client
// and returns an error if any
func (u *User) SendPacketToServer(packetType string, innerPacket any) error {

	u.mu.RLock()
	conn := u.Conn
	u.mu.RUnlock()

	ctx := context.Background()
	writeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// marshal inner packet to JSON
	innerBytes, marshalErr := json.Marshal(innerPacket)
	if marshalErr != nil {
		slog.Error("err_marshalling_inner_packet", "err", marshalErr)
		return marshalErr
	}

	// create envelope struct and wrap inner packet in it
	envelope := shared.GeneralPacket{
		Type:    packetType,
		Message: innerBytes,
	}

	// check if conn is alive and write envelope to connection
	if conn == nil {
		return fmt.Errorf("	err_conn_is_nil")
	}
	wsjson.Write(writeCtx, u.Conn, envelope)

	return nil
}

// StartClient establishes a connection to the server using the host and port provided.
// It returns an error if the host or port is empty.
func StartClient(host, port string) error {

	// check if host or port is empty, default to localhost:8080
	if shared.HasEmptyArgs(host, port) {
		slog.Warn("No host or port provided: Defaulting to localhost:8080")
		host = "localhost"
		port = "8080"
	}

	// Call crypto suite to generate a new key pair
	// stores the public and private keys in the user session
	err := NewUserSession()
	if err != nil {
		slog.Error("Error generating user session", "err", err)
		return err
	}

	// format the address string
	addr := fmt.Sprintf("ws://%s:%s", host, port)

	// create a new context with a timeout of 30 seconds
	rootCtx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// create a new user instance
	// initiates the conn and tuichan fields
	user := NewUser()
	user.Addr = addr
	go user.NewChatClient(rootCtx) // non-blocking

	//  TODO: call start TUI and pass in rootCtx

	return nil
}
