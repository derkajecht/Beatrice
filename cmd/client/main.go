package main

import (
	"flag"
	"sync"

	"github.com/derkajecht/Beatrice/internal/client/websocket"
)

func main() {

	// Parse command line arguments
	host := flag.String("host", "localhost", "Host of the server")
	port := flag.String("port", "8080", "Port of the server")

	flag.Parse()

	// start the client with the host and port provided
	// NOTE: can i put this in a goroutine?
	wg := new(sync.WaitGroup)
	wg.Add(1) // add a wait group to ensure the server is closed after the main function is done
	wg.Go(func() {
		websocket.StartClient(*host, *port)
	})
	wg.Wait() // wait for the server to close

	// example usage:
	// go run main.go -host localhost -port 8080 -contype tcp
}
