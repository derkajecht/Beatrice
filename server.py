from cryptography.hazmat.primitives.asymmetric import rsa, padding
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.backends import default_backend
import asyncio
import json
import random


class BeatriceServer:
    def __init__(self, host: str, port: int) -> None:
        self.host: str = host
        self.port: int = port

        # Data Structure Reference:
        # self.connected_users = {
        #    "Alice": {
        #        "writer": <asyncio.StreamWriter>,
        #        "key": "-----BEGIN PUBLIC KEY..."
        #    }
        # }
        self.connected_users = {}

    async def _receive_packet(self, reader) -> dict | None:
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
        except json.JSONDecodeError:  # json error if there is an invalid json
            return None

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
            await writer.write_closed()
            return

        # 2. Synchronise
        await self._synchronise(writer, new_nickname, new_key)

        # 3. Init message loop
        await self._message_loop(reader, nickname)

        # 4. Cleanup on disconnect
        await self._cleanup(nickname, writer)

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
                _type = packet.get("t")  # Type of packet recieved
                _nickname = packet.get("n")  # Nickname
                _key = packet.get("k")  # Public key

                if not all(
                    [_type, _nickname, _key]
                ):  # Checks to see that all fields are not none, if any data is missing, send the error packet to the user.
                    error_msg = "Missing required handshake packet data fields."  # Custom error message to send on failure
                    await self._send_packet(writer, {"t": "ERR", "c": error_msg})
                    return None

                # Check that the data packet contains the key information.
                if not _key.startswith("-----BEGIN PUBLIC KEY"):
                    error_msg = (
                        "Invalid public key."  # Set custom error message on failure
                    )
                    await self._send_packet(writer, {"t": "ERR", "c": error_msg})
                    return None

                try:
                    serialization.load_pem_public_key(
                        _key.encode("utf-8")
                    )  # If this passes, the key is mathematically valid
                except (
                    ValueError,
                    TypeError,
                ):  # If it does not pass, ValueError is raised, indicating that the public key is invalid.
                    error_msg = "Invalid public key. Invalid format or encoding. Please provide a valid PEM-encoded public key."
                    await self._send_packet(writer, {"t": "ERR", "c": error_msg})
                    return None

                try:
                    final_nickname = _nickname  # Check to make sure this is a nickname that isn't already in use. If it is, append a number and generate another one until you find a non-used name.
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

        current_users_list = (
            []
        )  # list of currently connected users, and their public keys. to be sent to new person.

        dir_packet = {  # Current users list packet. To be sent to the new person only.
            "t": "DIR",
            "p": current_users_list,
        }

        join_packet = (
            {  # Info on the recently joined person, to be sent to everyone else. Bosh
                "n": new_nickname,
                "k": new_key,
            }
        )

        for user in self.connected_users:
            if user != new_nickname:
                try:
                    self.connected_users[user].get("writer").write(
                        (json.dumps(join_packet) + "\n").encode("utf-8")
                    )
                    current_users_list.append(
                        {"n": user, "k": self.connected_users[user]["key"]}
                    )
                except:
                    pass

        try:
            await self._send_packet(writer, dir_packet)
        except Exception:
            pass

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
                packet = await self._receive_packet(reader)
            except Exception:
                break

            if packet is None:
                continue
            elif packet.get("t") == "M":
                recipient = packet.get("r")
                # message = packet.get("m")
                packet["s"] = nickname

                if recipient == "ALL":
                    for user in self.connected_users:
                        if user != nickname:
                            try:
                                await self._send_packet(
                                    self.connected_users[user].get("writer"), packet
                                )  # send to all
                            except:
                                pass
                else:
                    if recipient in self.connected_users:
                        if recipient != nickname:
                            try:
                                await self._send_packet(
                                    self.connected_users[recipient].get("writer"),
                                    packet,
                                )  # send to all
                            except:
                                pass

    async def _cleanup(self, nickname, writer):
        """
        ensures that data is removed from the connected_client list if a user disconnects.
        could use a simple cleanup packet to send to save bandwidth;
                {
                  "t": "L",          // Type: Left/Leave
                  "n": "Nickname"    // Who to remove
                }
        should also close the server if no connections are active
        """
        pass


# --- Execution ---
if __name__ == "__main__":
    server = BeatriceServer("127.0.0.1", 55556)
    try:
        asyncio.run(server.start_server())
    except KeyboardInterrupt:
        print("Server stopped.")
