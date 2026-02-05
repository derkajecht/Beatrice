import asyncio
import json
import random
import logging
from datetime import datetime
from cryptography.hazmat.primitives import serialization

# Logger setup
logger = logging.getLogger(__name__)
logging.basicConfig(
    level=logging.INFO, format="%(asctime)s - %(levelname)s - %(message)s"
)

# Constants
NICKNAME_SUFFIX_MIN = 100
NICKNAME_SUFFIX_MAX = 999
HANDSHAKE_TIMEOUT = 5


class BeatriceServer:
    def __init__(self, host: str, port: int) -> None:
        self.host: str = host
        self.port: int = port

        # Data Structure Reference:
        # self.connected_users = {
        #    "Alice": {
        #        "writer": <asyncio.StreamWriter>,
        #        "key": "-----BEGIN PUBLIC KEY..."
        #    },
        #    "Jordan": {
        #        "writer": <asyncio.StreamWriter>,
        #        "key": "-----BEGIN PUBLIC KEY..."
        #    },
        # }
        self.connected_users = {}

        self._users_lock = asyncio.Lock()

    async def start_server(self):
        # Start the bg task that checks for inactive users
        # asyncio.create_task(self._inactivity_check())

        server = await asyncio.start_server(
            self.handle_client,
            self.host,
            self.port,
            reuse_address=True,
            limit=256 * 1024,
        )

        async with server:
            await server.serve_forever()

    async def handle_client(self, reader, writer):
        """
        This will handle most of the server side logic and make sure everything is called in the right order.
        """
        # 1. Perform handshake
        nickname = await self._handshake(reader, writer)
        if not nickname:
            writer.close()
            await writer.wait_closed()
            return

        try:
            # 2. Synchronise
            key = self.connected_users[nickname]["key"]
            await self._synchronise(writer, nickname, key)

            # 3. Init message loop
            await self._message_loop(reader, nickname)

        except Exception as e:
            logger.error(f"Error: {e}")

        finally:
            # 4. Cleanup on disconnect
            await self._cleanup(nickname)

    async def _receive_packet(self, reader) -> None | dict[str, str]:
        """Method that listens for packets and correctly stops the server if the client leaves"""
        try:
            message: bytes = await reader.readline()
        except Exception as e:
            logger.error(f"Socket read error: {e}")
            raise e  # Re-raise so the loop knows to stop

        if message == b"":
            # EOF received. Raise exception to break the loop in the caller.
            raise ConnectionResetError("Client disconnected")

        if message == b"\n":
            await asyncio.sleep(0.1)
            return None

        try:
            packet = json.loads(message.decode("utf-8"))
            return packet
        except json.JSONDecodeError as e:
            logger.error(f"Invalid JSON {e}")
            return None

    async def _send_packet(self, writer, packet):
        """
        1. Encodes the packet to JSON bytes (with newline).
        2. Writes it to the specific writer.
        3. Drains the writer to ensure it sent.
        Safety: Wrap in try/except to ignore broken pipes.
        """
        try:
            writer.write((json.dumps(packet) + "\n").encode("utf-8"))
            await writer.drain()

        # If sending fails, the socket is likely closed
        except Exception as e:
            logger.error(f"Error: {e}")

    async def _handshake(self, reader, writer):
        """
        Read Line: Get the first packet.

        Parse JSON: If it's not JSON, kick them out.

        Check if we've received the correct packet type "H". If not, do not continue.

        Extract Key: Does the k (key) field exist?

        Validate Key: Does it look like a valid PEM key? (Starts with -----BEGIN PUBLIC KEY...).

        Success: Only then do you look at the nickname and let them in.
        """
        while True:
            try:
                packet = await asyncio.wait_for(
                    self._receive_packet(reader), timeout=HANDSHAKE_TIMEOUT
                )
            except asyncio.TimeoutError:
                logger.warning("Handshake timed out")
                return None

            if packet is None:
                await asyncio.sleep(0.1)
                continue

            elif packet.get("t") == "H":
                # Get the values from the decoded json packet
                _nickname = packet.get("n")  # Nickname
                _key = packet.get("k")  # Public key

                # Checks to see that all fields are not none, if any data is missing, send the error packet to the user.
                # Error packet structure for reference
                # {
                #   "t": "ERR",            // Type: Error
                #   "c": "User not found"  // Content: Description of the error
                # }

                if not all([_nickname, _key]):
                    error_msg = "Missing required handshake packet data fields."  # Custom error message to send on failure
                    err_packet = {"t": "ERR", "c": error_msg}
                    await self._send_packet(writer, err_packet)
                    return None

                # Check that the data packet contains the key information.
                if not _key.startswith("-----BEGIN PUBLIC KEY"):
                    error_msg = "Invalid public key."
                    err_packet = {"t": "ERR", "c": error_msg}
                    await self._send_packet(writer, err_packet)
                    return None

                loop = asyncio.get_running_loop()
                try:
                    # If this passes, the key is mathematically valid
                    await loop.run_in_executor(
                        None,
                        serialization.load_pem_public_key,
                        _key.encode("utf-8"),
                    )

                # If it does not pass, ValueError is raised, indicating that the public key is invalid.
                except Exception as e:
                    error_msg = "Invalid public key. Invalid format or encoding. Please provide a valid PEM-encoded public key."
                    await self._send_packet(writer, {"t": "ERR", "c": error_msg})
                    logger.error(f"Error (likely an invalid public key): {e}")
                    return None

                try:
                    # Check to make sure this is a nickname that isn't already in use. If it is, append a number and generate another one until you find a non-used name.
                    final_nickname = _nickname
                    if final_nickname in self.connected_users:
                        suffix = random.randint(
                            NICKNAME_SUFFIX_MIN, NICKNAME_SUFFIX_MAX
                        )
                        final_nickname = f"{_nickname}#{suffix}"
                    async with self._users_lock:
                        self.connected_users[final_nickname] = (
                            {  # Store this in the connected_users dict
                                "writer": writer,
                                "key": _key,
                                "last_activity": datetime.now(),
                                "status": "active",
                            }
                        )
                    self.latest_nickname = final_nickname
                    return final_nickname
                except Exception as e:
                    logger.error(f"Error setting nickname. {e}")
                    return None

            else:  # If the packet type doesn't begin with "H" which indicates its a handshake request.
                error_msg = "Error: Received non-handshake packet."  # Set custom error message on failure
                await self._send_packet(writer, {"t": "ERR", "c": error_msg})
                return None

    async def _synchronise(self, writer, new_nickname, new_key):
        """
        Action A: Sends the connected_users list to the new person incl public key.
        Action B: Loops through connected_users and sends a USER_JOINED packet to everyone else.

        will need to decode or encode these before sending
        """

        # Info on the recently joined person, to be sent to everyone else. Bosh
        join_packet = {
            "t": "J",
            "n": new_nickname,
            "k": new_key,
        }

        # Create full list of current connected users
        targets = []
        current_user_list = []

        async with self._users_lock:
            for user, data in self.connected_users.items():
                # send the dir_packet to the new user
                if user != new_nickname:
                    current_user_list.append({"n": user, "k": data["key"]})
                    targets.append(data["writer"])

        # dir_packet structure for reference
        # {
        #   "t": "DIR",            // Type: Directory
        #   "p": [                 // Payload: List of user objects
        #     {
        #       "n": "Bob",        // Nickname
        #       "k": "-----BEGIN..." // Public Key
        #     },
        #     {
        #       "n": "Charlie",
        #       "k": "-----BEGIN..."
        #     }
        #   ]
        # }

        dir_packet = {
            "t": "DIR",
            "p": current_user_list,
        }
        # send the packet to the new user
        await self._send_packet(writer, dir_packet)

        # Info on the recently joined person, to be sent to everyone else. Bosh

        tasks = [self._send_packet(t_writer, join_packet) for t_writer in targets]

        await asyncio.gather(*tasks, return_exceptions=True)

    async def _message_loop(self, reader, nickname):
        """
        This is an infinite loop which will continuously listen for messages and dispatch messages to the intended recipient(s).

        Make sure that a delimiter is used so the whole message is read properly asyncio.StreamReader()

        Handle Disconnection: If a client disconnects, make sure to remove them from connected_users list.

        {
            "t": "M",          # Type: Message
            "r": "recipient",  # Who is it for? (e.g., "Bob" or "ALL")
            "m": "..."         # The Message Content (Encrypted String)
        }

        """
        while True:  # continuously listen for incoming messages
            try:
                message_packet = await self._receive_packet(reader)
            except Exception:
                break

            if message_packet is None:
                continue

            elif message_packet.get("t") == "M":
                # Get recipient data
                recipient = message_packet.get("r")
                message_packet["s"] = nickname

                target_writer = None

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
                        logger.error(f"Error: {e}")
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
            print(f"----- Server: {nickname} has disconnected -----")
        # Is this needed for tui or could we use a logging system for the tui to handle???

        # Notify other users by sending the leave packet
        # Leave packet structure for reference
        # {
        #   "t": "L",              // Type: Leave
        #   "n": "Alice"           // Nickname of user who left
        # }
        leave_packet = {"t": "L", "n": nickname}  # Type: Leave  # Who is leaving

        # Iterate through the connected user list and send them the leave packet
        async with self._users_lock:
            for user in list(self.connected_users):
                try:
                    target_writer: asyncio.StreamWriter = self.connected_users[user][
                        "writer"
                    ]
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


# TODO: Feature that sets user status to "online" or "away" depending on the time they last sent a packet to the server
# This will require the client sending a packet to the server whenever they send a message or move the mouse.
# If the server receives that packet before a timeout length, a green circle will appear next to their name in the ui. If not, it will set them to away


# --- Execution ---
if __name__ == "__main__":

    # Pass host and port to __init__
    server = BeatriceServer("0.0.0.0", 55556)

    # Run the main async entry point
    try:
        asyncio.run(server.start_server())
    except KeyboardInterrupt:
        logger.info("Server stopped.")
        print("Server stopped.")
