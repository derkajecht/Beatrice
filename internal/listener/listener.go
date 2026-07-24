// Package listener handles incoming connections and packets and calls the appropriate
// function based on the packet type.
package listener

import (
	"io"
	"log/slog"
	"net"
	"sync"

	"github.com/derkajecht/Beatrice/internal/types"
	"github.com/derkajecht/Beatrice/internal/utils"
)

// bufferPool is a sync.Pool that creates buffers of size 4096
var bufferPool = sync.Pool{
	New: func() any {
		return make([]byte, 4096)
	},
}

// HandleConnection handles incoming connections and packets. It reads data from the connection,
// unmarshals it, and calls the appropriate function based on the packet type.
func HandleConnection(conn net.Conn) {
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
				return
			}
			slog.Error("Error reading from connection:", "err", err, "client", conn.RemoteAddr())
			conn.Close()
			return
		}

		// get envelope struct from buffer to unmarshal and check for packet type
		var genPacket types.GeneralPacket
		err = utils.UnmarshalPacket(conn, buf, n, &genPacket)
		if err != nil {
			return
		}

		// packet types used in the chat;
		// h = handshake, m = message, j = join, l = leave, e = error, d = dir, c = challenge

		// check packets based on type and Handle them accordingly
		// Handle... functions are defined in router.go
		switch genPacket.Type {

		// --- Handle handshake packet ---
		case "h":
			var p types.HandshakePacket
			err = utils.UnmarshalPacket(conn, buf, n, &p)
			if err != nil {
				return
			}

			// Call the handleHandshake function with the received handshake packet
			utils.HandleHandshake(p, conn)

		// --- Handle message packet ---
		case "m":
			var p types.MessagePacket
			err = utils.UnmarshalPacket(conn, buf, n, &p)
			if err != nil {
				return
			}

			// Call the handleMessage function with the received message packet
			utils.HandleMessage(p, conn)

		// --- Handle join packet ---
		case "j":
			var p types.JoinPacket
			err = utils.UnmarshalPacket(conn, buf, n, &p)
			if err != nil {
				return
			}

			// Call the handleJoin function with the received join packet
			utils.HandleJoin(p, conn)

		// --- Handle leave packet ---
		case "l":
			var p types.LeavePacket
			err = utils.UnmarshalPacket(conn, buf, n, &p)
			if err != nil {
				return
			}

			// Call the handleLeave function with the received leave packet
			utils.HandleLeave(p, conn)

		// --- Handle error packet ---
		case "e":
			var p types.ErrPacket
			err = utils.UnmarshalPacket(conn, buf, n, &p)
			if err != nil {
				return
			}

			// Call the handleError function with the received error packet
			utils.HandleError(p, conn)

		// --- Handle dir packet ---
		case "d":
			var p types.DirPacket
			err = utils.UnmarshalPacket(conn, buf, n, &p)
			if err != nil {
				return
			}

			// Call the handleDir function with the received dir packet
			utils.HandleDir(p, conn)

		// --- Handle challenge packet ---
		case "c":
			var p types.ChallengePacket
			err = utils.UnmarshalPacket(conn, buf, n, &p)
			if err != nil {
				return
			}

			// Call the handleChallenge function with the received challenge packet
			utils.HandleChallenge(p, conn)
		}

		// clear the buffer after use and put it back into the pool
		clear(buf)
		bufferPool.Put(buf)

	}
}
