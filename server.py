from cryptography.hazmat.primitives.asymmetric import rsa, padding
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.backends import default_backend
import asyncio
import json
import random


class BeatriceServer:
    def __init__(self, host: str, port: int) -> None:
        self.host = host
        self.port = port
        # Data Structure Reference:
        # self.connected_users = {
        #    "Alice": {
        #        "writer": <asyncio.StreamWriter>,
        #        "key": "-----BEGIN PUBLIC KEY..."
        #    }
        # }
        self.connected_users = {}

    async def start_server(self):
        server = await asyncio.start_server(self.handle_client, self.host, self.port)

        async with server:
            await server.serve_forever()

    async def handle_client(self, reader, writer):
        """
        This will handle most of the server side logic and make sure its called in the right order.
        """
        pass

    async def _handshake(self, reader, writer):
        """
        Read Line: Get the first packet.

        Parse JSON: If it's not JSON, kick them out.

        Check if weve received the correct packet type "H". If not, do not continue.

        Extract Key: Does the k (key) field exist?

        Validate Key: Does it look like a valid PEM key? (Starts with -----BEGIN PUBLIC KEY...).

        Success: Only then do you look at the nickname and let them in.
        """
        try:
            message = await asyncio.wait_for(reader.readline(), timeout=5.0)
            if not message:
                return None
        except asyncio.TimeoutError:
            return None

        try:
            message_str = message.decode("utf-8")
        except UnicodeDecodeError:
            return None

        try:
            handshake_packet = json.loads(message_str) # decrypt the utf8 message into JSON packet if its valid json
        except json.JSONDecodeError: # json error if there is an invalid json
            return None

        # Get the values from the decoded json packet
        _type = handshake_packet.get("t") # Type of packet recieved
        _nickname = handshake_packet.get("n") # Nickname
        _key = handshake_packet.get("k") # Public key

        if not all([_type, _nickname, _key]): # Checks to see that all fields are not none, if any data is missing, send the error packet to the user.
            error_msg = "Missing required handshake packet data fields." # Custom error message to send on failure
            writer.write((json.dumps({"t": "ERR", "c": error_msg}) + "\n").encode("utf-8")) # Send the error packet
            await writer.drain() # Await for the write to drain
            return None

        if _type != "H": # If the packet type begins with "H" which indicates its a handshake request.
            error_msg = "Error: Received non-handshake packet." # Set custom error message on failure
            writer.write((json.dumps({"t": "ERR", "c": error_msg}) + "\n").encode("utf-8")) # Send the error packet
            await writer.drain() # Await for write to drain
            return None

        # Check that the data packet contains the key information.
        if not _key.startswith("-----BEGIN PUBLIC KEY"):
            error_msg = "Invalid public key." # Set custom error message on failure
            writer.write((json.dumps({"t": "ERR", "c": error_msg}) + "\n").encode("utf-8")) # Send the error packet
            await writer.drain() # Await for the write to drain
            return None

        try:
            serialization.load_pem_public_key( _key.encode('utf-8')) # If this passes, the key is mathematically valid
        except (ValueError, TypeError): # If it does not pass, ValueError is raised, indicating that the public key is invalid.
            error_msg = "Invalid public key. Invalid format or encoding. Please provide a valid PEM-encoded public key."
            writer.write((json.dumps({"t": "ERR", "c": error_msg}) + "\n").encode("utf-8")) # Send the error packet
            await writer.drain() # Await for the write to drain
            return None

        try:
            final_nickname = _nickname # Check to make sure this is a nickname that isn't already in use. If it is, append a number and generate another one until you find a non-used name.
            if final_nickname in self.connected_users:
                suffix = random.randint(100,999)
                final_nickname = f"{_nickname}#{suffix}"
            self.connected_users[final_nickname] = { # Store this in the connected_users dict
                "writer":writer,
                "key":_key
            }
            return final_nickname
        except Exception:
            return None


    async def _synchronise(self, writer, new_nickname, new_key):
        """
        Action A: Sends the connected_users list to the new person incl public key.
        Action B: Loops through connected_users and sends a USER_JOINED packet to everyone else.

        will need to decode or encode these before sending
        """
        pass

    async def _message_loop(self, reader, sender_nickname):
        """
        This is an infinite loop which will continuously listen for messages and dispatch messages to the intended recipient(s).

        Make sure that a delimiter is used so the whole message is read properly asyncio.StreamReader()

        Handle Disconnection: If a client disconnects, make sure to remove them from connected_users list.
        """
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
