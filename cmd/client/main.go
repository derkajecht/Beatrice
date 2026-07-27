package main

import (
	"flag"
	"sync"

	"github.com/derkajecht/Beatrice/internal/api"
	"github.com/derkajecht/Beatrice/internal/client/crypto"
)

// StartClient establishes a connection to the server using the host and port provided.
// It returns an error if the host or port is empty.
func StartClient(host, port, conType string) {

	// Call crypto suite to generate a new key pair
	// stores the public and private keys in the user session
	_, _, _, err := crypto.NewUserSession()
	if err != nil {
		println("Error generating user session:", err)
		return
	}
	// Open client connection to the server using the host and port provided
	// NewClient checks for empty host and port, and returns an error if either is empty
	err = api.NewClient(host, port, conType)
	if err != nil {
		println("Error creating client:", err)
		return
	}

	// TODO: call to start the tui
}

func main() {

	// Parse command line arguments
	host := flag.String("host", "localhost", "Host of the server")
	port := flag.String("port", "8080", "Port of the server")
	conType := flag.String("conType", "tcp", "Connection type (tcp, udp, etc.)")

	flag.Parse()

	// start the client with the host and port provided
	// NOTE: can i put this in a goroutine?
	wg := new(sync.WaitGroup)
	wg.Add(1) // add a wait group to ensure the server is closed after the main function is done
	wg.Go(func() {
		StartClient(*host, *port, *conType)
	})
	wg.Wait() // wait for the server to close

	// example usage:
	// go run main.go -host localhost -port 8080 -contype tcp
}
