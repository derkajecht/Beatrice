package utils

import (
	"encoding/json"
	"log"
	"net"

	"github.com/derkajecht/Beatrice/internal/models"
)

func SendError(conn net.Conn, errMsg string) bool {
	// Send an error packet to the client

	// set up error packet struct
	errPacket := models.ErrPacket{
		Type:    "e",
		Message: errMsg,
	}

	// marshal error packet struct to JSON
	buf, marshalErr := json.Marshal(errPacket)
	if marshalErr != nil {
		log.Println("Error marshalling error packet:", marshalErr)
		return false
	}

	// write JSON to connection
	_, writeErr := conn.Write(buf)
	if writeErr != nil {
		log.Println("Error writing to connection:", writeErr)
		return false
	}

	return true
}
