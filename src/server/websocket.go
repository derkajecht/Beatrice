package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/derkajecht/Beatrice/src/shared"
)

// addClient adds a client to the hub clients map
func (h *Hub) addClient(c *ServerClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c.ID] = c
}

func (h *Hub) getClient(id string) *ServerClient {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.clients[id]
}

// removeClient removes a client from the hub clients map
func (h *Hub) removeClient(c string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, c)
}

// Listener is the main listener for the hub
func (h *Hub) Listener(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-h.broadcastChn:
			var packet shared.GeneralPacket
			err := json.Unmarshal(msg.data, &packet)
			if err != nil {
				slog.Error("Error unmarshalling message packet", "err", err)
				continue
			}

			switch packet.Type {
			case "h":
				if err := HandleHandshake(h, msg.conn, packet); err != nil {
					slog.Error("error handling handshake", "err", err)
					_ = SendPacketToClient(msg.conn, "e", &shared.ErrPacket{
						Message: "err_handshake_failed",
					})
				}
			case "c":
				if err := HandleChallenge(h, msg.conn, packet); err != nil {
					slog.Error("error handling challenge", "err", err)
					_ = SendPacketToClient(msg.conn, "e", &shared.ErrPacket{
						Message: "err_challenge_failed",
					})
				}
			case "m":
				if err := HandleMessage(h, msg.conn, packet); err != nil {
					slog.Error("error handling message", "err", err)
				}
			case "j":
				if err := HandleJoin(h, msg.conn, packet); err != nil {
					slog.Error("error handling join", "err", err)
				}
			case "l":
				if err := HandleLeave(h, msg.conn, packet); err != nil {
					slog.Error("error handling leave", "err", err)
				}
			default:
				slog.Error("unknown packet type", "type", packet.Type)
			}
		}
	}
}

// wsHandler handles incoming websocket connections
func wsHandler(ctx context.Context, c *ServerClient, h *Hub) {
	// register the client synchronously first
	h.addClient(c)

	defer func() {
		h.removeClient(c.ID)
		c.Conn.Close(websocket.StatusNormalClosure, "")
	}()

	// keepalive: ping the client at InactivityTimeout/3 intervals
	go func() {
		interval := max(h.InactivityTimeout/3, time.Second)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				pctx, cancel := context.WithTimeout(ctx, 5*time.Second)
				err := c.Conn.Ping(pctx)
				cancel()
				if err != nil {
					slog.Debug("ping failed, closing connection", "client", c.ID, "err", err)
					c.Conn.Close(websocket.StatusGoingAway, "ping timeout")
					return
				}
			}
		}
	}()

	// read messages from the client synchronously
	for {
		readCtx, cancel := context.WithTimeout(ctx, h.InactivityTimeout)
		var m json.RawMessage
		err := wsjson.Read(readCtx, c.Conn, &m)
		cancel()
		if err != nil {
			return
		}
		h.broadcastChn <- broadcastMsg{conn: c, data: []byte(m)}
	}
}

// NewServerHandler constructs the HTTP mux for websocket connections.
func NewServerHandler(ctx context.Context, hub *Hub) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{})
		if err != nil {
			slog.Error("Error accepting websocket connection", "err", err)
			return
		}
		conn.SetReadLimit(1 << 20) // 1 MiB
		c := NewServerClient(r.RemoteAddr, conn)
		go wsHandler(ctx, c, hub)
	})
	return mux
}

// RunServer starts a new server instance and a new database connection
// It takes the host, port, and database name as arguments
// It returns an error if the host, port, or database name is empty
func StartServer(host, port, dbName, dbLocation string) {

	// check for empty args and log a warning if they are
	if shared.HasEmptyArgs(host, port, dbName, dbLocation) {
		slog.Warn("No host, port, db name or db location provided: Defaulting to localhost:8080, beatrice.db, and ./beatrice")
		host = "localhost"
		port = "8080"
		dbName = "beatrice.db"
		dbLocation = "~/beatrice"
	}

	// create server context
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	dbInfo, err := NewDatabase(dbName, dbLocation)
	if err != nil {
		log.Fatalf("Could not set up database: %v\n", err)
	}
	defer dbInfo.DB.Close()

	slog.Info("Database connection established")

	// register websocket and hub
	hub := NewHub()
	hub.db = dbInfo

	// start the listener goroutine
	go hub.Listener(ctx)

	mux := NewServerHandler(ctx, hub)

	addr := fmt.Sprintf("%s:%s", host, port)
	server := http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// shut down the http server when ctx is cancelled (ctrl+c, SIGTERM)
	go func() {
		<-ctx.Done()
		slog.Info("Shutting down server...")
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelShutdown()
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("Error during shutdown", "err", err)
		}
	}()

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("Error starting server", "err", err)
	}
}
