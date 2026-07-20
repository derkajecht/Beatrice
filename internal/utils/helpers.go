// Package utils provides utility functions for the server and client
package utils

import (
	"encoding/json"
	"log/slog"
	"net"
)

// func ClearBuffer(buffer []byte) {
// 	clear(buffer)
// for i := range buffer {
// 	buffer[i] = 0
// }
// }

// UnmarshalPacket unmarshals the given buffer and returns the envelope struct and error
// this as passed to the handle function to handle the packet based on the packet type
func UnmarshalPacket(conn net.Conn, buf []byte, n int, env any) error {
	if err := json.Unmarshal(buf[:n], env); err != nil {
		// log error
		slog.Error("Error unmarshalling envelope:", "err", err, "client", conn.RemoteAddr())

		// Send error packet to client
		SendError(conn, "invalid_protocol_format")

		return err
	}
	return nil
}
