package api

import (
	"log"
	"net"

	"github.com/derkajecht/Beatrice/internal/utils"
)

func NewServer(port int) {
	// TODO: Implement new server logic
	listener, _ := net.Listen("tcp", string(port))
	log.Printf("Server listening on port %d/n", port)

	for {
		conn, _ := listener.Accept()
		go utils.HandleConnection(conn)
	}
}
