package utils

import (
	"encoding/json"
	"log"
	"net"

	"github.com/derkajecht/Beatrice/internal/types"
)

// TODO: Could rename SendStatus to capture both success and error packets
// SendError sends a error packet to the client
func SendError(conn net.Conn, errMsg string) bool {
	// set up error packet struct
	errPacket := types.ErrPacket{
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

// SendPacketToClient sends a packet or message to the client
// and returns an error if any
// TODO: Could make the type of outboundMessage better
func SendPacketToClient(conn net.Conn, outboundMessage map[string]string) error {
	clientToReceive := types.GeneralPacket{
		Type:    "g",
		Message: outboundMessage,
	}

	buf, marshalErr := json.Marshal(clientToReceive)
	if marshalErr != nil {
		log.Println("Error marshalling success packet:", marshalErr)
		return marshalErr
	}

	_, writeErr := conn.Write(buf)
	return writeErr
}
