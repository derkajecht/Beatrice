package api

import (
	"fmt"
	"log"
	"net"

	"github.com/derkajecht/Beatrice/internal/utils"
)

// NewServer initializes a new server instance
// and listens for incoming connections on the specified port.
// It uses a goroutine to handle each incoming packet.
func NewServer(port string, host string) error {

	// Check if the port and host are empty
	inputs := map[string]string{
		"port": port,
		"host": host,
	}
	for fieldName, addr := range inputs {
		if utils.IsEmpty(addr) {
			return fmt.Errorf("Cannot start server: %s cannot be empty", fieldName)
		}
	}

	// Join the host and port strings to create the address
	address := net.JoinHostPort(host, port)

	// Listen for incoming connections on the specified port
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("Failed to listen on port %s: %w", port, err)
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
		go utils.HandleConnection(conn)
	}
}

func NewClient(port string, host string) error {

	// Check if the port and host are empty
	inputs := map[string]string{
		"port": port,
		"host": host,
	}
	for fieldName, addr := range inputs {
		if !utils.IsEmpty(addr) {
			return fmt.Errorf("Cannot connect to server: %s cannot be empty", fieldName)
		}
	}

	// Join the host and port strings to create the address
	address := net.JoinHostPort(host, port)

	// Listen for incoming connections on the specified port
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("Failed to listen on port %s: %w", port, err)
	}

	log.Printf("Listening on %s\n", address)

	// clean up on func exit
	defer listener.Close()

	for {
		// accept incoming connections
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Error accepting connection:", err)
			continue
		}

		// handle the connection in a new goroutine
		go utils.HandleConnection(conn)
	}
}
