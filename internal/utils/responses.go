package utils

import (
	"encoding/json"
	"io"
	"log/slog"
	"net"

	"github.com/derkajecht/Beatrice/internal/types"
)

// TODO: Could rename SendStatus to capture both success and error packets
// SendError sends a error packet to the client
func SendError(conn net.Conn, errMsg string) bool {
	// set up error packet struct
	errPacket := types.ErrPacket{
		Message: errMsg,
	}

	// marshal error packet struct to JSON
	buf, marshalErr := json.Marshal(errPacket)
	if marshalErr != nil {
		slog.Error("Error marshalling error packet:", "err", marshalErr, "client", conn.RemoteAddr())
		return false
	}

	// write JSON to connection
	_, writeErr := conn.Write(buf)
	if writeErr != nil {
		slog.Error("Error writing error packet to connection:", "err", writeErr, "client", conn.RemoteAddr())
		return false
	}

	return true
}

// SendPacketToClient sends a packet or message to the client
// and returns an error if any
func SendPacketToClient(conn net.Conn, packetType string, innerPacket any) error {

	// marshal inner packet to JSON
	innerBytes, marshalErr := json.Marshal(innerPacket)
	if marshalErr != nil {
		slog.Error("err_marshalling_inner_packet", "err", marshalErr, "client", conn.RemoteAddr())
		return marshalErr
	}

	// create envelope struct and wrap inner packet in it
	envelope := types.GeneralPacket{
		Type:    packetType,
		Message: innerBytes,
	}

	// marshal envelope struct to JSON
	finalPayload, marshalErr := json.Marshal(envelope)
	if marshalErr != nil {
		slog.Error("err_marshalling_envelope", "err", marshalErr, "client", conn.RemoteAddr())
		return marshalErr
	}

	// write the envelope to the connection
	n, writeErr := conn.Write(finalPayload)
	if writeErr != nil {
		slog.Error("err_writing_packet", "err", writeErr, "client", conn.RemoteAddr())
		return writeErr
	}

	// check if the entire packet was written
	// if not, return an error
	if n < len(finalPayload) {
		slog.Error("err_writing_packet", "err", writeErr, "client", conn.RemoteAddr())
		return io.ErrShortWrite
	}

	return nil
}

// DisconnectAndQuit closes the connection and quits the application
func DisconnectAndQuit(conn net.Conn) {
	SendError(conn, "err_connection_closed")
	conn.Close()
}
