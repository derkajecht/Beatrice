package server

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/derkajecht/Beatrice/internal/shared"
)

func ServerStruct(dbLocation string) *Server {
	return &Server{
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
			h.clients[client] = &ServerClient{}

		case client, ok := <-h.deleteClient:
			if !ok {
				// channel was/is closed
				return
			}
			delete(h.clients, client)
			client.CloseNow()

			// TODO: implement the handling of the receieved packets
			// case client := <-h.handshake:
			// 	// call handshake function
			// 	switch client.Type {
			// 	case "handshake":
			// 		HandleHandshake(client, h.clients[client])
			// 	case "challenge":
			// 		HandleChallenge(client, h.clients[client])
			// 	case "message":
			// 		HandleMessage(client, h.clients[client])
			// 	case "join":
			// 		HandleJoin(client, h.clients[client])
			// 	case "leave":
			// 		HandleLeave(client, h.clients[client])
			// 	case "error":
			// 		HandleError(client, h.clients[client])
			// 	case "dir":
			// 		HandleDir(client, h.clients[client])
			// 	}
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
	defer conn.Close(websocket.StatusNormalClosure, "")

	// send conn to the hub for handshake etc
	h.register <- conn

	// defer closing the connection
	defer func() {
		h.deleteClient <- conn
	}()

	// start listener goroutine to handle incoming packets accordingly
	ctx := r.Context()

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
	if shared.HasEmptyArgs(host, port, dbName, dbLocation) {
		slog.Warn("No host, port, db name or db location provided: Defaulting to localhost:8080, beatrice.db, and ./beatrice")
	}
	host = "localhost"
	port = "8080"
	dbName = "beatrice.db"
	dbLocation = "./beatrice"

	db, dbLocation, err := NewDatabase(dbName, dbLocation)
	if err != nil {
		log.Fatalf("Could not set up database: %v\n", err)
	}
	defer db.Close()
	// store the database path in the server struct
	ServerStruct(dbLocation)
	// log that the database connection was established
	slog.Info("Database connection established")

	hub := NewHub()

	// register websocket
	addr := fmt.Sprintf("%s:%s", host, port)
	http.HandleFunc("/ws", hub.wsHandler) // NOTE: not sure if correct to call NewHub().wshandler
	slog.Info("Starting server", "addr", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		slog.Error("Error starting server", "err", err)
	}
}
