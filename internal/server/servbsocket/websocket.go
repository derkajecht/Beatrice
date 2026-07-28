package websocket

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/derkajecht/Beatrice/internal/client/websocket"
	"github.com/derkajecht/Beatrice/internal/server/router"
	"github.com/derkajecht/Beatrice/internal/server/storage"
	"github.com/derkajecht/Beatrice/internal/shared/sharedvalidation"
	"github.com/derkajecht/Beatrice/internal/shared/types"
)

type Hub struct {
	clients      map[*websocket.Conn]
	register     chan *websocket.Conn
	broadcast    chan []byte
	dm           chan []byte
	handshake    chan *websocket.Conn
	deleteClient chan *websocket.Conn
}

func NewHub() *Hub {
	return &Hub{
		clients:      make(map[*websocket.Conn]*websocket.Client),
		broadcast:    make(chan []byte),
		dm:           make(chan []byte),
		handshake:    make(chan *websocket.Conn),
		deleteClient: make(chan *websocket.Conn),
	}
}

func ServerStruct(dbLocation string) *types.Server {
	return &types.Server{
		ActiveConnections:  make(map[string]map[string]string),
		PendingConnections: make(map[string]string),
		DatabasePath:       dbLocation,
	}
}

func (h *Hub) Listener(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case client := <-h.register:
			// add client to the map
			h.clients[client] = &types.Client{}

		case client, ok := <-h.deleteClient:
			if !ok {
				// channel was/is closed
				return
			}
			delete(h.clients, client)
			client.CloseNow()

		case client := <-h.handshake:
			// call handshake function
			switch client.Type {
			case "handshake":
				router.HandleHandshake(client, h.clients[client])
			case "challenge":
				router.HandleChallenge(client, h.clients[client])
			case "message":
				router.HandleMessage(client, h.clients[client])
			case "join":
				router.HandleJoin(client, h.clients[client])
			case "leave":
				router.HandleLeave(client, h.clients[client])
			case "error":
				router.HandleError(client, h.clients[client])
			case "dir":
				router.HandleDir(client, h.clients[client])
			}
		}
	}
}

// wsHandler handles incoming websocket connections
// it distributes the connection to the appropriate channels
func (h *Hub) wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		slog.Error("Error accepting websocket connection", "err", err)
		return
	}

	// start listener goroutine to handle incoming packets accordingly
	ctx := conn.CloseRead(r.Context())
	go h.Listener(ctx)

	// send conn to the hub for handshake etc
	h.register <- conn

	// defer closing the connection
	defer func() {
		h.deleteClient <- conn
	}()

	// blocks until a message is received
	for {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		_, msg, err := conn.Read(ctx)
		cancel()
		if err != nil {
			slog.Error("Error reading from websocket connection", "err", err)
			break
		}

		// send msg to the hub
		h.broadcast <- msg
	}
}

// RunServer starts a new server instance and a new database connection
// It takes the host, port, and database name as arguments
// It returns an error if the host, port, or database name is empty
func StartServer(host, port, dbName, dbLocation string) {

	// check for empty args and log a warning if they are
	if sharedvalidation.HasEmptyArgs(host, port, dbName, dbLocation) {
		slog.Warn("No host, port, db name or db location provided: Defaulting to localhost:8080, beatrice.db, and ./beatrice")
	}

	db, dbLocation, err := storage.NewDatabase(dbName, dbLocation)
	if err != nil {
		log.Fatalf("Could not set up database: %v\n", err)
	}
	defer db.Close()
	// store the database path in the server struct
	ServerStruct(dbLocation)
	// log that the database connection was established
	slog.Info("Database connection established")

	// register websocket
	addr := fmt.Sprintf("%s:%s", host, port)
	http.HandleFunc("/ws", wsHandler) // todo: change to hub.wsHandler
	slog.Info("Starting server", "addr", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		slog.Error("Error starting server", "err", err)
	}
}
