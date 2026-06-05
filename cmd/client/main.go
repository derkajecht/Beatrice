package main

import (
	"flag"

	"github.com/derkajecht/Beatrice/internal/api"
)

// main is the entry point for the client. It takes in the host and port of the destination server
// establishes a connection, and then sends a handshake message to the server. Once the handshake
// is successful, the client can send and receive messages from the server.
// It also calls the TUI to display the show the chat.
func StartClient(host string, port string) {

	// Open client connection to the server using the host and port provided
	// NewClient checks for empty host and port, and returns an error if either is empty
	err := api.NewClient(host, port)
	if err != nil {
		println("Error creating client:", err)
		return
	}
}

func main() {
	// TODO: check that this would work well with the TUI

	// Parse command line arguments
	host := flag.String("host", "localhost", "Host of the server")
	port := flag.String("port", "8080", "Port of the server")

	flag.Parse()

	// start the client with the host and port provided
	StartClient(*host, *port)

	// example usage:
	// go run main.go -host localhost -port 8080
}
