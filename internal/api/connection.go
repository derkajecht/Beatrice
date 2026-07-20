// Package api provides the main calls for starting and connecting to the server and client.
package api

import (
	"fmt"
	"log"
	"log/slog"
	"net"

	"github.com/derkajecht/Beatrice/internal/config"
	listenerpkg "github.com/derkajecht/Beatrice/internal/listener"
	"github.com/derkajecht/Beatrice/internal/validation"
)

// NewServer initializes a new server instance
// and listens for incoming connections on the specified port.
// It uses a goroutine to handle each incoming packet.
func NewServer(host, port, conType string) error {

	// Check if the port and host are empty
	// if yes, default values are set automatically
	if validation.IsArgsEmpty(host, port, conType) {
		slog.Warn("No host, port or connection type provided: Defaulting to localhost:8080 and tcp")
	}

	// create a new connection info struct
	cfg := config.NewConnectionInfo()

	// assign the port, host & connection type to the server struct so it can be used later and in
	// different functions
	cfg.Host = host
	cfg.Port = port
	cfg.ConType = conType

	// Join the host and port strings to create the address
	address := net.JoinHostPort(cfg.Host, cfg.Port)

	// Listen for incoming connections on the specified port
	listener, err := net.Listen(cfg.ConType, address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s:%s, %s: %w", cfg.Host, cfg.Port, cfg.ConType, err)
	}

	log.Printf("Listening on %s\n", address)

	// clean up on func exit
	defer listener.Close()

	for {
		// accept incoming connections
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}

		// handle the connection in a new goroutine
		go listenerpkg.HandleConnection(conn)
	}
}

// NewClient establishes a connection to the server using the host, port and connection type provided.
func NewClient(host, port, conType string) error {

	// Check if the port and host are empty
	// if yes, default values are set automatically
	if validation.IsArgsEmpty(host, port, conType) {
		slog.Warn("No host, port or connection type provided: Defaulting to localhost:8080 and tcp")
	}

	// create a new connection info struct
	cfg := config.NewConnectionInfo()
	cfg.Host = host
	cfg.Port = port
	cfg.ConType = conType

	// Join the host and port strings to create the address
	address := net.JoinHostPort(cfg.Host, cfg.Port)

	// conn.Close() called inside listenerpkg.HandleConnection. No need to close here.
	for {
		// Call net.Dial to connect to the server
		conn, err := net.Dial(cfg.ConType, address)
		if err != nil {
			return fmt.Errorf("failed to connect to server %s:%s, %s: %w", cfg.Host, cfg.Port, cfg.ConType, err)
		}

		// handle the connection in a new goroutine (see listenerpkg.HandleConnection)
		go listenerpkg.HandleConnection(conn)
	}
}
