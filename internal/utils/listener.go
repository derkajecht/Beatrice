package utils

import (
	"encoding/json"
	"log"
	"net"

	"github.com/derkajecht/Beatrice/internal/types"
)

func HandleConnection(conn net.Conn) {
	// close the connection once the function is done (ie. when the app is closed)
	defer conn.Close()

	// create buffer to store raw data
	buf := make([]byte, 4096)

	// loop until connection is closed
	// listen for incoming data and Handle it accordingly
	for {

		// read data from connection
		// if an error occurs, log it and return
		n, err := conn.Read(buf)
		if err != nil {
			log.Println("Error reading from connection:", err)
			return
		}

		// get envelope struct from buffer to unmarshal and check for packet type
		var env types.Envelope

		if err := json.Unmarshal(buf[:n], &env); err != nil {
			log.Println("Error unmarshalling envelope:", err)
			return
		}

		// packet types used in the chat;
		// h = handshake, m = message, j = join, l = leave, e = error, d = dir, c = challenge

		// check packets based on type and Handle them accordingly
		// Handle... functions are defined in router.go
		switch env.Type {
		// Handle handshake packet
		case "h":
			var p types.HandshakePacket
			if err := json.Unmarshal(buf[:n], &p); err != nil {
				log.Println("Error unmarshalling handshake packet:", err)
				return
			}

			// Call the handleHandshake function with the received handshake packet
			HandleHandshake(p, conn)

		// Handle message packet
		case "m":
			var p types.MessagePacket
			json.Unmarshal(buf[:n], &p)
			HandleMessage(p, conn) // Call the handleMessage function with the received message packet

		// Handle join packet
		case "j":
			var p types.JoinPacket
			json.Unmarshal(buf[:n], &p)
			HandleJoin(p, conn) // Call the handleJoin function with the received join packet

		// Handle leave packet
		case "l":
			var p types.LeavePacket
			json.Unmarshal(buf[:n], &p)
			HandleLeave(p, conn) // Call the handleLeave function with the received leave packet

		// Handle error packet
		case "e":
			var p types.ErrPacket
			json.Unmarshal(buf[:n], &p)
			HandleError(p, conn) // Call the handleError function with the received error packet

		// Handle dir packet
		case "d":
			var p types.DirPacket
			json.Unmarshal(buf[:n], &p)
			HandleDir(p, conn) // Call the handleDir function with the received dir packet

		// Handle challenge packet
		case "c":
			var p types.ChallengePacket
			json.Unmarshal(buf[:n], &p)
			HandleChallenge(p, conn) // Call the handleChallenge function with the received challenge packet
		}

	}
}
