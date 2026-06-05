package listener

import (
	"encoding/json"
	"log/slog"
	"net"
	"sync"

	"github.com/derkajecht/Beatrice/internal/types"
	"github.com/derkajecht/Beatrice/internal/utils"
)

var bufferPool = sync.Pool{
	New: func() any {
		return make([]byte, 4096)
	},
}

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
			slog.Error("Error reading from connection:", "err", err, "client", conn.RemoteAddr())
			return
		}

		// get envelope struct from buffer to unmarshal and check for packet type
		var env types.GeneralPacket
		if err := json.Unmarshal(buf[:n], &env); err != nil {
			// log error
			slog.Error("Error unmarshalling envelope:", "err", err, "client", conn.RemoteAddr())

			// Send error packet to client
			utils.SendError(conn, "invalid_protocol_format")

			continue
		}

		// packet types used in the chat;
		// h = handshake, m = message, j = join, l = leave, e = error, d = dir, c = challenge

		// check packets based on type and Handle them accordingly
		// Handle... functions are defined in router.go
		switch env.Type {

		// --- Handle handshake packet ---
		case "h":
			var p types.HandshakePacket
			if err := json.Unmarshal(buf[:n], &p); err != nil {
				// log error
				slog.Error("Error unmarshalling handshake packet:", "err", err, "client", conn.RemoteAddr())

				// Send error packet to client
				utils.SendError(conn, "invalid_protocol_format")

				break
			}

			// Call the handleHandshake function with the received handshake packet
			utils.HandleHandshake(p, conn)

		// --- Handle message packet ---
		case "m":
			var p types.MessagePacket
			if err := json.Unmarshal(buf[:n], &p); err != nil {
				// log error
				slog.Error("Error unmarshalling message packet:", "err", err, "client", conn.RemoteAddr())

				// Send error packet to client
				utils.SendError(conn, "invalid_protocol_format")

				break
			}
			// Call the handleMessage function with the received message packet
			utils.HandleMessage(p, conn)

		// --- Handle join packet ---
		case "j":
			var p types.JoinPacket
			if err := json.Unmarshal(buf[:n], &p); err != nil {
				// log error
				slog.Error("Error unmarshalling join packet:", "err", err, "client", conn.RemoteAddr())

				// Send error packet to client
				utils.SendError(conn, "invalid_protocol_format")

				break
			}

			// Call the handleJoin function with the received join packet
			utils.HandleJoin(p, conn)

		// --- Handle leave packet ---
		case "l":
			var p types.LeavePacket
			if err := json.Unmarshal(buf[:n], &p); err != nil {
				// log error
				slog.Error("Error unmarshalling leave packet:", "err", err, "client", conn.RemoteAddr())

				// Send error packet to client
				utils.SendError(conn, "invalid_protocol_format")

				break
			}
			// Call the handleLeave function with the received leave packet
			utils.HandleLeave(p, conn)

		// --- Handle error packet ---
		case "e":
			var p types.ErrPacket
			if err := json.Unmarshal(buf[:n], &p); err != nil {
				// log error
				slog.Error("Error unmarshalling error packet:", "err", err, "client", conn.RemoteAddr())

				// Send error packet to client
				utils.SendError(conn, "invalid_protocol_format")

				break
			}
			// Call the handleError function with the received error packet
			utils.HandleError(p, conn)

		// --- Handle dir packet ---
		case "d":
			var p types.DirPacket
			if err := json.Unmarshal(buf[:n], &p); err != nil {
				// log error
				slog.Error("Error unmarshalling dir packet:", "err", err, "client", conn.RemoteAddr())

				// Send error packet to client
				utils.SendError(conn, "invalid_protocol_format")

				break
			}
			// Call the handleDir function with the received dir packet
			utils.HandleDir(p, conn)

		// --- Handle challenge packet ---
		case "c":
			var p types.ChallengePacket
			if err := json.Unmarshal(buf[:n], &p); err != nil {
				// log error
				slog.Error("Error unmarshalling challenge packet:", "err", err, "client", conn.RemoteAddr())

				// Send error packet to client
				utils.SendError(conn, "invalid_protocol_format")

				break
			}
			// Call the handleChallenge function with the received challenge packet
			utils.HandleChallenge(p, conn)
		}

		// clear the buffer after use and put it back into the pool
		utils.ClearBuffer(buf)
		bufferPool.Put(buf)

	}
}
