from cryptography.hazmat.primitives.asymmetric import rsa, padding
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.backends import default_backend
import asyncio
import json
import random
import logging

# Logger setup
logger = logging.getLogger(__name__)
logging.basicConfig(
    level=logging.INFO, format="%(asctime)s - %(levelname)s - %(message)s"
)


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
        self.connected_users: dict[str, dict[str, asyncio.StreamWriter | str]] = {}

    async def _receive_packet(self, reader) -> None | dict[str, str]:
        try:
            message: str = await reader.readline()
            if (
                message == b""
            ):  # if an empty string is received, this means the client has disconnected.
                raise Exception("Client has disconnected. Connection will be closed.")
            elif not message or message == b"\n":
                return None  # if no message is received or an empty message is received, do nothing

            packet = json.loads(
                message.decode("utf-8")
            )  # decrypt the utf8 message into JSON packet if its valid json
            return packet
        except json.JSONDecodeError as e:  # json error if there is an invalid json
            logger.error(f"Invalid JSON {e}. Message will be skipped")
            pass

    async def _send_packet(self, writer, packet):
        """
        1. Encodes the packet to JSON bytes (with newline).
        2. Writes it to the specific writer.
        3. Drains the writer to ensure it sent.
        Safety: Wrap in try/except to ignore broken pipes.
        """
        try:
            writer.write(
                (json.dumps(packet) + "\n").encode("utf-8")
            )  # Send the error packet
            await writer.drain()
        except Exception:  # If an exception occurs
            pass

    async def start_server(self):
        server = await asyncio.start_server(self.handle_client, self.host, self.port)

        async with server:
            await server.serve_forever()

    async def handle_client(self, reader, writer):
        """
        This will handle most of the server side logic and make sure its called in the right order.
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

        finally:
            # 4. Cleanup on disconnect
            await self._cleanup(nickname)

    async def _handshake(self, reader, writer):
        """
        Read Line: Get the first packet.

        Parse JSON: If it's not JSON, kick them out.

        Check if weve received the correct packet type "H". If not, do not continue.

        Extract Key: Does the k (key) field exist?

        Validate Key: Does it look like a valid PEM key? (Starts with -----BEGIN PUBLIC KEY...).

        Success: Only then do you look at the nickname and let them in.
        """
        while True:
            try:
                packet = await asyncio.wait_for(self._receive_packet(reader), timeout=5)
            except Exception:
                break

            if packet is None:
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

                try:
                    # If this passes, the key is mathematically valid
                    serialization.load_pem_public_key(_key.encode("utf-8"))

                # If it does not pass, ValueError is raised, indicating that the public key is invalid.
                except (ValueError, TypeError):
                    error_msg = "Invalid public key. Invalid format or encoding. Please provide a valid PEM-encoded public key."
                    await self._send_packet(writer, {"t": "ERR", "c": error_msg})
                    return None

                try:
                    # Check to make sure this is a nickname that isn't already in use. If it is, append a number and generate another one until you find a non-used name.
                    final_nickname = _nickname
                    if final_nickname in self.connected_users:
                        suffix = random.randint(100, 999)
                        final_nickname = f"{_nickname}#{suffix}"
                    self.connected_users[final_nickname] = (
                        {  # Store this in the connected_users dict
                            "writer": writer,
                            "key": _key,
                        }
                    )
                    self.latest_nickname = final_nickname
                    return final_nickname
                except Exception:
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
        current_user_list = []
        for user in self.connected_users:
            # send the dir_packet to the new user
            if user != new_nickname:
                current_user_list.append(
                    {"n": user, "k": self.connected_users[user]["key"]}
                )

        # --- Send current connected user info to the newly connected user ---

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

        # --- Broadcast the join_packet to all other connected users ---
        # Info on the recently joined person, to be sent to everyone else. Bosh

        # Join packet structure for reference
        # {
        #   "t": "J",              // Type: Join
        #   "n": "Alice",          // Nickname
        #   "k": "-----BEGIN..."   // Public Key
        # }

        join_packet = {
            "t": "J",
            "n": new_nickname,
            "k": new_key,
        }

        # Send the join packet to all users except for the most recently joined user
        for user in self.connected_users:
            if user != new_nickname:
                target_writer = self.connected_users[user]["writer"]
                await self._send_packet(target_writer, join_packet)

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
                recipient = message_packet.get("r")
                message_packet["s"] = nickname

                if recipient == "ALL":
                    for user in self.connected_users:
                        if user != nickname:
                            try:
                                asyncio.create_task(
                                    self._send_packet(
                                        self.connected_users[user].get("writer"),
                                        message_packet,
                                    )
                                )  # send to all
                            except:
                                pass
                else:
                    if recipient in self.connected_users:
                        if recipient != nickname:
                            try:
                                await self._send_packet(
                                    self.connected_users[recipient].get("writer"),
                                    message_packet,
                                )  # send to all
                            except:
                                pass
                    else:
                        error_packet = {
                            "t": "ERR",
                            "c": f"User '{recipient}' not found or disconnected",
                        }
                        await self._send_packet(
                            self.connected_users[nickname]["writer"], error_packet
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

        # Get the writer of the user we need to remove
        writer_to_close = self.connected_users[nickname]["writer"]

        # Remove user from the connected_user list
        del self.connected_users[nickname]
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
        for user in self.connected_users:
            try:
                target_writer = self.connected_users[user]["writer"]
                await self._send_packet(target_writer, leave_packet)
            except Exception:
                pass

        # Close the socket of the disconnected user
        try:
            writer_to_close.close()
            await writer_to_close.wait_closed()
        except Exception:
            pass


# --- Execution ---
if __name__ == "__main__":

    # Pass host and port to __init__
    server = BeatriceServer("127.0.0.1", 55556)

    # Run the main async entry point
    try:
        asyncio.run(server.start_server())
    except KeyboardInterrupt:
        print("Server stopped.")
