package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/derkajecht/Beatrice/internal/shared"
)

func HubInit(dbLocation string) *Hub {
	return &Hub{
		DatabasePath: dbLocation,
	}
}

// addClient adds a client to the hub clients map
func (h *Hub) addClient(c *ServerClient) {
	h.clients[c.ID] = c
}

// removeClient removes a client from the hub clients map
func (h *Hub) removeClient(c string) {
	delete(h.clients, c)
}

// Listener is the main listener for the hub
func (h *Hub) Listener(ctx context.Context, c *ServerClient) {
	for {
		select {
		case <-ctx.Done():
			return
		case client := <-h.addClientChn:
			// add client to the map
			h.addClient(client)
		case client := <-h.removeClientChn:
			h.removeClient(client.ID)

		// TODO: implement the handling of the receieved packets
		case msg := <-h.broadcastChn:
			// unmarshal the message to json
			var packet shared.GeneralPacket
			err := json.Unmarshal(msg, &packet)
			if err != nil {
				slog.Error("Error unmarshalling message packet", "err", err)
				break
			}
			slog.Info("packet successfully unmarshalled", "packet", packet)

			// call handshake function
			switch packet.Type {
			case "h": // handshake
				// TODO: finish writing the handshake function
				if err := HandleHandshake(packet); err != nil {
					slog.Error("error handling handshake", "err", err)
					// send failure packet back to client to trigger tui event
					err := h.SendPacketToClient(c, "err", &shared.ErrPacket{
						Message: "err_handshake_failed",
					})
					if err != nil {
						slog.Error("error sending handshake failure packet", "err", err)
					}
				}

			// case "challenge":
			// 	HandleChallenge(client, h.clients[client])
			// 	case "message":
			// 		HandleMessage(client, h.clients[client])
			// 	case "join":
			// 		HandleJoin(client, h.clients[client])
			// 	case "leave":
			// 		HandleLeave(client, h.clients[client])
			// 	case "error":
			// 		HandleError(client, h.clients[client])
			default:
				slog.Error("unknown packet type", "packet", packet)
			}
		}
	}
}

// wsHandler handles incoming websocket connections
// it distributes the connection to the appropriate channels
func wsHandler(ctx context.Context, c *ServerClient, h *Hub) {
	// close the connection when the TUI exits
	defer func() {
		h.removeClientChn <- c
		c.Conn.Close(websocket.StatusNormalClosure, "")
	}()
	// register the client in the hub
	h.addClientChn <- c
	// start the listener
	go h.Listener(ctx, c)
	// start the broadcast loop
	// sends all messages received from the client to the hub via the broadcast channel
	for {
		var m []byte
		err := wsjson.Read(ctx, c.Conn, &m)
		if err != nil {
			slog.Error("error in Receive Message: ", err.Error())
			break
		}
		h.broadcastChn <- m
	}
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
		dbLocation = "./beatrice"
	}

	db, dbLocation, err := NewDatabase(dbName, dbLocation)
	if err != nil {
		log.Fatalf("Could not set up database: %v\n", err)
	}
	defer db.Close()
	// store the database path in the server struct
	HubInit(dbLocation)
	// log that the database connection was established
	slog.Info("Database connection established")

	// register websocket and hub
	hub := NewHub()
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			slog.Error("Error accepting websocket connection", "err", err)
			return
		}
		remoteAddr := r.RemoteAddr
		client := &ServerClient{
			ID:      remoteAddr,
			Conn:    conn,
			PubKey:  []byte{},
			TuiChan: make(chan []byte),
		}
		hub.addClient(client)
		defer hub.removeClient(client)

		wsHandler(r.Context(), client, hub)
	})

	addr := fmt.Sprintf("%s:%s", host, port)
	server := http.Server{
		Addr:    addr,
		Handler: mux,
	}
	if err := server.ListenAndServe(); err != nil {
		slog.Error("Error starting server", "err", err)
	}
}
