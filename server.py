import argparse
import asyncio
import json
import logging
import random
from base64 import encode
from datetime import datetime

import uvicorn
from cryptography.hazmat.primitives import serialization
from fastapi import FastAPI, WebSocket

# Logger setup
logger = logging.getLogger(__name__)
logging.basicConfig(
    level=logging.INFO, format="%(asctime)s - %(levelname)s - %(message)s"
)

NICKNAME_SUFFIX_MIN = 100
NICKNAME_SUFFIX_MAX = 999

app = FastAPI()

class ConnectionManager:
    """ Handles client connect and disconnect. """
    def __init__(self):
        # Dict of current active users - "jordan": {"ws": 1234, "pk": "-----BEGIN PUBLIC KEY..."}
        self.active_connections: dict[str, dict] = {}

    async def connect(self, nickname, websocket, public_key):
        """ Start websockt and add user to active connections. """
        # Accept websocket connection
        await websocket.accept()
        # Add information to the dict to store user connection data
        self.active_connections[nickname] = {"ws": websocket, "pk": public_key}
        logger.info(f"{nickname} joined the server")

    async def disconnect(self, nickname):
        """ Remove user from active connections. """
        self.active_connections.pop(nickname, None)
        logger.info(f"{nickname} left the server")

    async def broadcast_message(self, message_payload: dict, exclude = None):
        """ Send message to all connected users. """
        for nickname, data in self.active_connections.items():
            if nickname != exclude:
                await data["ws"].send_json(message_payload)

    async def dm_message(self, message_payload, target):
        """ Send message to specific user. """
        if target in self.active_connections:
            dm_target = self.active_connections[target]["ws"]
            await dm_target.send_json(message_payload)
            return True
        return False

manager = ConnectionManager()

@app.websocket("/ws")
async def beatrice(websocket: WebSocket):
    try:
        # Listen for incoming json from the client
        packet = await websocket.receive_json()

        # if the type of packet is not handshake, return and close the connection
        if packet.get("t") != "H":
            err_packet = {"t": "ERR", "c": "Invalid handshake"}
            await websocket.send_json(err_packet)
            await websocket.close(0, "Invalid handshake from client.")
            return

        # Extract from packet
        _nickname = packet.get("n")  # Nickname
        _key = packet.get("k")  # Public key

        # If this passes, the public key is valid
        try:
            serialization.load_pem_public_key(_key.encode("utf-8"))
        except Exception:
            err_packet = {"t": "ERR", "c": "Invalid PEM public key."}
            await websocket.send_json(err_packet)
            await websocket.close()
            return

        # check for duplicate nicknames. adds random suffix to the end to avoid dups
        if _nickname in manager.active_connections:
            suffix = random.randint(NICKNAME_SUFFIX_MIN, NICKNAME_SUFFIX_MAX)
            _nickname = f"{_nickname}#{suffix}"

        # Make connection via websocket
        await manager.connect(_nickname, websocket, _key)

        # Send dir packet
        # Get current active user list
        current_users = [{"n": n, "k": d["pk"]} for n, d in manager.active_connections.items() if n != _nickname]
        # create dir packet
        dir_packet = {"t": "DIR", "p": current_users}
        await websocket.send_json(dir_packet)

        # Send join packet
        join_packet = {"t": "J", "n": _nickname, "k": _key}
        await manager.broadcast_message(join_packet, exclude=_nickname)

        # MESSAGE LOOP
        while True:  # continuously listen for incoming messages
            message_packet = await websocket.receive_json()

            if message_packet.get("t") == "M":
                # DM LOGIC ---
                if message_packet.get("r") != "ALL":
                    # Get recipient data
                    recipient = message_packet.get("r")
                    if recipient not in manager.active_connections:
                        logger.warning(f"{recipient} not found.")

                    # Sign the message
                    message_packet["s"] = _nickname

                    success = await manager.dm_message(message_packet, recipient)
                    if not success:
                        err_packet = {"t": "ERR", "c": f"Error sending message to {recipient}"}
                        await websocket.send_json(err_packet)
                success = await manager.broadcast_message(message_packet, exclude=_nickname)
                if not success:
                    err_packet = {"t": "ERR", "c": f"Error broadcasting message"}
                    await websocket.send_json(err_packet)

                # TODO: Finish rewriting logic over to fastapi
                # Need to add in cleanup method when a user disconnects
                # Add in error handling




                async with self._users_lock:
                    if recipient in self.connected_users:
                        # Copy reference to the writer
                        target_writer = self.connected_users[recipient]["writer"]

                # If the writer is found, send the message packet to it
                if target_writer:
                    try:
                        await self._send_packet(
                            self.connected_users[recipient].get("writer"),
                            message_packet,
                        )
                    except Exception as e:
                        logger.error(f"Error target writer not found: {e}")
                        return
                else:
                    # Send error packet if the writer is not found
                    error_packet = {
                        "t": "ERR",
                        "c": f"User '{recipient}' not found.",
                    }
                    await self._send_packet(
                        reader._transport.get_protocol()._stream_writer, error_packet
                    )

    async def _cleanup(self, nickname):
        """
        ensures that data is removed from the connected_client list if a user disconnects.
        could use a simple cleanup packet to send to save bandwidth;
                {
                  "t": "L",          // Type: Left/Leave
                  "n": "Nickname"    // Who to remove
                }
        should also close the server if no connections are active
        """

        # Handles an edge case that if the user is not already in the connected_users list, we dont need to do anything
        if nickname not in self.connected_users:
            return

        logger.info(f"{nickname} disconnecting...")

        # Get the writer of the user we need to remove
        writer_to_close = self.connected_users[nickname]["writer"]

        # Remove user from the connected_user list
        async with self._users_lock:
            del self.connected_users[nickname]
            logger.info(f"----- Server: {nickname} has disconnected -----")
            # print(f"----- Server: {nickname} has disconnected -----")

        # Notify other users by sending the leave packet
        # Leave packet structure for reference
        # {
        #   "t": "L",              // Type: Leave
        #   "n": "Alice"           // Nickname of user who left
        # }
        leave_packet = {"t": "L", "n": nickname}  # Type: Leave  # Who is leaving

        # TODO: write better algo to send this faster than O(1) - not sure its possible
        # Iterate through the connected user list and send them the leave packet
        async with self._users_lock:
            for user in self.connected_users.keys():
                target_writer = self.connected_users[user]["writer"]
                try:
                    await self._send_packet(target_writer, leave_packet)
                except Exception as e:
                    logger.error(f"Error: {e}")
                    pass

        # Close the socket of the disconnected user
        try:
            writer_to_close.close()
            await writer_to_close.wait_closed()
        except Exception as e:
            logger.error(f"Error: {e}")
            pass

# --- Execution ---
if __name__ == "__main__":

    parser = argparse.ArgumentParser(
        prog="Beatrice Server",
        description="Start a Beatrice server",
        add_help=False,
        conflict_handler="resolve",
    )
    parser.add_argument(
        "--host",
        type=str,
        default="0.0.0.0",
        help="IP to bind the server to (default: '0.0.0.0')",
    )
    parser.add_argument(
        "--port",
        type=int,
        default=55556,
        help="Port to bind the server to (default: 55556)",
    )

    parser.add_argument(
        "-h",
        "--help",
        action="help",
        default=argparse.SUPPRESS,
        help="Show this help message",
    )

    args = parser.parse_args()

    # Run the main async entry point
    try:
        # asyncio.run(server.start_server(args.host, args.port))
        uvicorn.run(app, host=args.host, port=args.port)
    except KeyboardInterrupt:
        logger.info("Server stopped.")
        print("Server stopped.")
