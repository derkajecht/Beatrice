package utils

import (
	"encoding/json"
	"io"
	"log/slog"
	"net"

	"github.com/derkajecht/Beatrice/internal/shared/types"
)

// TODO: Could rename SendStatus to capture both success and error packets

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
	SendPacketToClient(conn, "q", &types.ErrPacket{
		Message: "err_connection_closed",
	})
	conn.Close()
}
