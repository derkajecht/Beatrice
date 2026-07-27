// Package listener handles incoming connections and packets and calls the appropriate
// function based on the packet type.
package listener

import (
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"sync"

	"github.com/derkajecht/Beatrice/internal/server/router"
	"github.com/derkajecht/Beatrice/internal/server/validation"
	"github.com/derkajecht/Beatrice/internal/shared/types"
)

// bufferPool is a sync.Pool that creates buffers of size 4096
var bufferPool = sync.Pool{
	New: func() any {
		return make([]byte, 4096)
	},
}

// HandleConnection handles incoming connections and packets. It reads data from the connection,
// unmarshals it, and calls the appropriate function based on the packet type.
func HandleConnection(conn net.Conn) error {
	// close the connection once the function is done (ie. when the app is closed)
	defer conn.Close()

	// loop until connection is closed
	// listen for incoming data and Handle it accordingly
	for {

		// create buffer to store raw data
		buf := bufferPool.Get().([]byte)

		// read data from connection
		// if an error occurs, log it and return
		n, err := conn.Read(buf)
		if err != nil {
			if err == io.EOF {
				slog.Info("Connection closed by client", "client", conn.RemoteAddr())
				return nil
			}
			slog.Error("Error reading from connection:", "err", err, "client", conn.RemoteAddr())
			conn.Close()
			bufferPool.Put(buf)
			return nil
		}

		// get envelope struct from buffer to unmarshal and check for packet type
		var genPacket types.GeneralPacket
		err = validation.UnmarshalPacket(conn, buf, n, &genPacket)
		if err != nil {
			return nil
		}

		// pass to Dispatch function to handle the packet
		Dispatch(conn, genPacket)

		// clear the buffer after use and put it back into the pool
		clear(buf)
		defer bufferPool.Put(buf)
	}
}

func Dispatch(conn net.Conn, gen types.GeneralPacket) {
	// packet types used in the chat;
	// h = handshake, m = message, j = join, l = leave, e = error, d = dir, c = challenge

	switch gen.Type {
	// --- Handle handshake packet ---
	case "h":
		var p types.HandshakePacket
		if err := json.Unmarshal(gen.Message, &p); err != nil {
			slog.Error("Error unmarshalling handshake packet", "err", err)
		}

		// Call the handleHandshake function with the received handshake packet
		router.HandleHandshake(p, conn)

	// --- Handle message packet ---
	case "m":
		var p types.MessagePacket
		if err := json.Unmarshal(gen.Message, &p); err != nil {
			slog.Error("Error unmarshalling message packet", "err", err)
		}

		// Call the handleMessage function with the received message packet
		router.HandleMessage(p, conn)

	// --- Handle join packet ---
	case "j":
		var p types.JoinPacket
		if err := json.Unmarshal(gen.Message, &p); err != nil {
			slog.Error("Error unmarshalling join packet", "err", err)
		}

		// Call the handleJoin function with the received join packet
		router.HandleJoin(p, conn)

	// --- Handle leave packet ---
	case "l":
		var p types.LeavePacket
		if err := json.Unmarshal(gen.Message, &p); err != nil {
			slog.Error("Error unmarshalling leave packet", "err", err)
		}

		// Call the handleLeave function with the received leave packet
		router.HandleLeave(p, conn)

	// --- Handle error packet ---
	case "e":
		var p types.ErrPacket
		if err := json.Unmarshal(gen.Message, &p); err != nil {
			slog.Error("Error unmarshalling error packet", "err", err)
		}

		// Call the handleError function with the received error packet
		router.HandleError(p, conn)

	// --- Handle dir packet ---
	case "d":
		var p types.DirPacket
		if err := json.Unmarshal(gen.Message, &p); err != nil {
			slog.Error("Error unmarshalling error packet", "err", err)
		}

		// Call the handleDir function with the received dir packet
		router.HandleDir(p, conn)

	// --- Handle challenge packet ---
	case "c":
		var p types.ChallengePacket
		if err := json.Unmarshal(gen.Message, &p); err != nil {
			slog.Error("Error unmarshalling challenge packet", "err", err)
		}

		// Call the handleChallenge function with the received challenge packet
		router.HandleChallenge(p, conn)
	}
}
