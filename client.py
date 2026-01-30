import os
import logging
import base64
import asyncio
import json
import hashlib

from typing import Union, List, Tuple, Any, Optional

# Cryptography imports
from cryptography.hazmat.primitives.asymmetric import rsa, padding
from cryptography.hazmat.primitives import serialization, hashes
from cryptography.hazmat.primitives.ciphers.aead import AESGCM
from cryptography.hazmat.backends import default_backend
from cryptography.hazmat.primitives.asymmetric.rsa import RSAPrivateKey, RSAPublicKey

# Type definitions
Event = Union[
    Tuple[str, str, str],  # ("message", sender, content)
    Tuple[str, str],  # ("error", message)
    Tuple[str, List[str]],  # ("directory_update", users)
]

# Logger setup
logger = logging.getLogger(__name__)
logging.basicConfig(
    level=logging.INFO, format="%(asctime)s - %(levelname)s - %(message)s"
)

# Set max nickname length at 20 chars
MAX_NICKNAME_LENGTH = 20


class Client:
    def __init__(self, host: str, port: int, nickname: str) -> None:
        # /--- Store host & port, set nickname
        self.host: str = host
        self.port: int = port
        self.nickname: str = nickname
        # ---/

        # /--- Aysncio shtuff ---
        self._reader: Optional[asyncio.StreamReader] = None
        self.writer: Optional[asyncio.StreamWriter] = None

        # Queue for sending events to the UI
        self.event_queue: asyncio.Queue[Event] = asyncio.Queue()
        # ---/

        # Init connected status as false since we are not connected yet
        self.connected: bool = False

        # --- CRYPTO SETUP ---
        # /--- Public and private key generation
        self.private_key: RSAPrivateKey = rsa.generate_private_key(
            public_exponent=65537, key_size=2048, backend=default_backend()
        )
        self.public_key: RSAPublicKey = self.private_key.public_key()
        # ---/

        # /--- Serialize public key for transmission
        public_key_bytes = self.public_key.public_bytes(
            encoding=serialization.Encoding.PEM,
            format=serialization.PublicFormat.SubjectPublicKeyInfo,
        )
        self.public_key_str: str = public_key_bytes.decode("utf-8")
        # ---/

        # Store other users public keys
        self.user_public_keys: dict[str, RSAPublicKey] = {}

        # /--- Init handshake packet data
        self.handshake_packet: dict[str, str] = {
            "t": "H",
            "n": self.nickname,
            "k": self.public_key_str,
        }
        # ---/

    @property
    def reader(self) -> asyncio.StreamReader:
        """
        Property to safely access the asyncio StreamReader.

        Returns:
            asyncio.StreamReader: The connection reader stream

        Raises:
            ValueError: If reader hasn't been initialized yet
        """
        if self._reader is None:
            raise ValueError("reader attribute not set")
        return self._reader

    @reader.setter
    def reader(self, value: asyncio.StreamReader) -> None:
        self._reader = value

    async def connect_to_server(self, host: str, port: int):
        """
        Establish TCP connection to the chat server. - could use udp down the line for fun?

        Creates asyncio Reader and Writer for bidirectional
        communication with the server.

        Args:
            host: Server IP address or hostname
            port: Server port number

        Raises:
            ConnectionError: If unable to connect to server

        Time Complexity: O(1) - single connection attempt
        """
        try:
            self._reader, self.writer = await asyncio.open_connection(
                host, port
            )  # Import host & port from server side logic
        except ConnectionRefusedError:
            logger.error(f"Could not connect to host and port.")
            raise Exception("Unable to connect to host and port.")

    # helper method to avoid re-writing the same lines of code over and over
    async def _send_packet(self, packet: dict[str, str]):
        """
        1. Encodes the packet to JSON bytes (with newline).
        2. Writes it to the specific writer.
        3. Drains the writer to ensure it sent.
        Safety: Wrap in try/except to ignore broken pipes.
        """
        try:
            # Encodes the packet to bytes and writes it to the writer, followed by a newline.
            assert self.writer is not None
            self.writer.write((json.dumps(packet) + "\n").encode("utf-8"))

            # Drains the writer's buffer, ensuring that all data has been sent.
            assert self.writer is not None
            await self.writer.drain()

        # If an exception occurs
        except Exception as e:
            logger.warning(f"Error sending packet: {e}")
            return

    # helper method to avoid re-writing the same lines of code over and over
    async def _receive_packet(self) -> dict[str, Any] | None:
        """
        1. Receives incoming data from the network stream
        2. Checks if data is empty, which indicates no new data is available
        3. Convert the received data into JSON format using json.loads() and return it
        Safety: Wrap in try/except to ignore broken pipes.
        """
        try:
            # Receive messages and assign to a variable.
            # The message comes in as bytes. decode to utf-8 string format for processing.
            message = await self.reader.readline()

            # if an empty string is received, this means the client has disconnected.
            if message == b"":
                logger.error("Client disconnected")
                raise Exception("Client has disconnected. Connection will be closed.")

            # if no message is received or an empty message is received, do nothing
            elif not message or message == b"\n":
                pass

            try:
                # decrypt the utf8 message into JSON packet if its valid json
                packet: dict[str, str] = json.loads(message.decode("utf-8"))
                return packet

            # json error if there is an invalid json
            except json.JSONDecodeError as e:
                logger.warning(f"Invalid JSON received: {e}")
                pass

        # generic exception for any other error not covered above
        except Exception as e:
            logger.warning(f"Error receiving packet: {e}")
            return

    def store_user_public_key(self, nickname: str, public_key_str: str):
        """
        Stores the given user's public key in a dictionary with the nickname as the key. This allows for quick lookup of other users' public keys when sending encrypted messages.

        parameters:
        - nickname (str): The nickname associated with the user whose public key is being stored.

        """
        try:
            public_key = serialization.load_pem_public_key(
                public_key_str.encode("utf-8"), backend=default_backend()
            )
            self.user_public_keys[nickname] = public_key
        except Exception as e:
            logger.error(f"Error storing public key for {nickname}: {e}")

    def get_fingerprint(self, nickname) -> str:
        """
        Generate a short, readable hex representation of the fingerprint for a given public key
        """

        # If the nickname is not in public keys, return "Unknown"
        if nickname not in self.user_public_keys:
            return "Unknown"

        # Get the public key for that user
        key = self.user_public_keys[nickname]

        # Serialize it to bytes
        key_bytes = key.public_bytes(
            encoding=serialization.Encoding.PEM,
            format=serialization.PublicFormat.SubjectPublicKeyInfo,
        )

        # Hash the key bytes using SHA-256
        sha = hashlib.sha256(key_bytes).hexdigest()

        # return the first 4 characters and last 4 characters of sha256 hash
        return f"{sha[:4]}:{sha[4:8]}"

    @reader.setter
    def reader(self, value: asyncio.StreamReader) -> None:
        self._reader = value

    async def check_handshake(self) -> bool:
        """
        Async method to check the client handshake response. If successful, the client public key is stored in the Client class object so messages can be sent to and from the connected client(s).

        If no key is received, then the chat is closed as encryption can't be guaranteed.

        Returns:
            True if handshake successful, False otherwise.
        """

        # Send the Handshake Packet immediately
        try:
            await self._send_packet(self.handshake_packet)

        # If there is an error while sending the packet, then log and exit
        except Exception as e:
            logger.error(
                f"Error sending handshake: {e}! Exiting. Please try reconnecting. "
            )
            return False

        # Listen for response (The server might send ERR or DIR)
        # If successful, Server sends nothing specific back immediately,
        # it just proceeds to _synchronise and sends the DIR packet.
        try:
            response_packet = await self._receive_packet()

            if not response_packet:
                pass

            if response_packet and response_packet.get("t") == "ERR":
                # get the actul error message from the error packet
                error_message = response_packet.get("c")
                logger.error(
                    f"Error received: {error_message}! Exiting. Please try reconnecting."
                )
                return False

            # if we receive a dir packet. get the data from it.
            elif response_packet and response_packet.get("t") == "DIR":
                # Get the list of connected users from the dir packet, send by the server and store them in a var
                connected_users_list = response_packet.get("p", [])

                for user_data in connected_users_list:
                    nickname = user_data.get("n")
                    public_key_info = user_data.get("k")

                    if nickname and public_key_info:
                        self.store_user_public_key(nickname, public_key_info)

                # --- add in logic to get the correct info from the received dir packet
                # --- need to add the nickname and public key info for each user that is currently connected

        except Exception as e:
            logger.error(
                "Error receiving response from the server! Exiting. Please try reconnecting. "
            )
            return False

        return True

    async def receive_messages(self) -> None:
        """
        Continuously listens for packets from the server and handles them.

        Processes different packet types: messages, errors, directory updates, etc.
        """
        while True:

            # Async receiving to avoid blocking the main thread
            packet = await self._receive_packet()

            # If packet is None, it means the connection was closed. Break out of loop.
            if not packet:
                break

            # Get the type of packet
            packet_type = packet.get("t")

            # TODO: Could refactor this by splitting the decryption part into its own method, might look cleaner
            # --- MESSAGE PACKET ---
            if packet_type == "M":
                # Handle message
                sender = packet.get("s")
                encrypted_msg_b64 = packet.get("m")
                encrypted_key_b64 = packet.get("k")
                iv_b64 = packet.get("iv")

                # check to see if all parts of the message packet have been received, if not, chuck em out and log an error
                if not all([sender, encrypted_msg_b64, encrypted_key_b64, iv_b64]):
                    logger.error("Message packet incomplete")
                    continue

                try:
                    # Decode from base64
                    encrypted_key = base64.b64decode(encrypted_key_b64)
                    encrypted_message = base64.b64decode(encrypted_msg_b64)
                    iv = base64.b64decode(iv_b64)

                    # Decrypt the aes key with the private rsa key
                    aes_key = self.private_key.decrypt(
                        encrypted_key,
                        padding.OAEP(
                            mgf=padding.MGF1(algorithm=hashes.SHA256()),
                            algorithm=hashes.SHA256(),
                            label=None,
                        ),
                    )

                    # decrypt the content via aes
                    aesgcm = AESGCM(aes_key)

                    decrypted_content_bytes = aesgcm.decrypt(
                        iv, encrypted_message, None
                    )

                    # decrypt the json content using utf-8 and json.load
                    decrypted_json_str = decrypted_content_bytes.decode("utf-8")

                    with open("debug_log.txt", "a") as f:
                        f.write(
                            f"DEBUG: Type: {type(decrypted_json_str)}, Content: '{decrypted_json_str}'\n"
                        )

                    message_payload = json.loads(decrypted_json_str)

                    # Send to tui
                    await self.event_queue.put(("message", message_payload))

                # Skip invalid messages
                except Exception as e:
                    logger.error(f"Error decrypting message: {e}")
                    continue

            # --- JOIN PACKET ---
            elif packet_type == "J":
                # Extract the new nickname and key from the packet
                new_nick, new_key = packet.get("n"), packet.get("k")

                if new_nick and new_key:
                    self.store_user_public_key(new_nick, new_key)

                    # update the tui with the name of the new user
                    await self.event_queue.put(
                        ("join_packet", f"{new_nick} has joined the chat!")
                    )

            # --- DIR PACKET ---
            elif packet_type == "DIR":
                connected_users_list = packet.get("p", [])

                for user_data in connected_users_list:
                    nick, key = user_data.get("n"), user_data.get("k")

                    if nick and key:
                        self.store_user_public_key(nick, key)

                await self.event_queue.put(
                    (
                        "dir",
                        f"Connected. Found {len(connected_users_list)} users in the chat.",
                    )
                )

            # --- LEAVE PACKET ---
            elif packet_type == "L":
                # assign nickname of user who is leaving to a variable
                left_nick = packet.get("n")

                # if that user is inside the public keys dict, delete them from it
                if left_nick in self.user_public_keys:
                    del self.user_public_keys[left_nick]

                    # update the tui
                    await self.event_queue.put(
                        ("leave_packet", f"{left_nick} has left the chat")
                    )

            # --- ERROR PACKET ---
            elif packet_type == "ERR":
                error_message = packet.get("c")

                # update the tui
                await self.event_queue.put(("err", error_message))

    async def send_message(self, content: str):
        # - Collect input from the user (must be non-blocking)
        # - Checks to see if the message is intended for a specific person, if not, broadcast to all. Will need to loop through current connected_users if sending to all.
        # - Remove whitespace at the start and end of each message
        # - Encrypt the message with the AES key.
        # - Encrypt the AES key with the recipients public key. Might be complex loop if sending to more than one user.
        # - Build the message packet;
        # {
        #   "t": "M",              // Type: Message
        #   "r": "Bob",            // Recipient: "Nickname" or "ALL"
        #   "s": "Alice",          // Sender (Added by Server)
        #
        #   "iv": "...",           // AES Initialization Vector (Base64 String)
        #   "k": "...",          // The AES Key, encrypted with Recipient's RSA Public Key (Base64 String)
        #   "m": "..."             // The Message Content, encrypted with the AES Key (Base64 String)
        # }
        # - Send the packet via _send_packet method

        # if the user enters quit or exit, stop the program
        if content.strip().lower() in ["exit", "quit"]:
            await self.event_queue.put(("status", "You have disconnected."))
            return

        # Assume that the message is being sent to everyone initially
        recipient = "ALL"
        if not content:
            return
        else:
            content = content.strip()

        # Check for DM
        if content.startswith("@"):
            # Will split nickname from the rest of the message so we can use that to encrypt the message with the intended recipients key.
            parts = content[1:].split(" ", 1)
            if len(parts) == 2:
                # Update the recipient variable with the intended user
                recipient = parts[0].strip()

                # Add in check to make sure you cant send message to yourself
                if recipient == self.nickname:
                    await self.event_queue.put(
                        ("self_message_error", " You cannot send message to yourself.")
                    )
                    logger.error(
                        f"Cannot send a direct message to yourself. Please try again."
                    )
                    return

                # Update the content varible with the message content
                content = parts[1].strip()

            else:  # If message doesn't follow the correct syntax let the user know
                await self.event_queue.put(
                    ("dm_usage_error", "Usage: @Nickname Message")
                )
                return
        else:
            pass

        payload_dict = {"sender": self.nickname, "content": content}

        payload_json_str = json.dumps(payload_dict)

        # Generate AES and IV for encryption
        aes_key = AESGCM.generate_key(bit_length=256)
        aesgcm = AESGCM(aes_key)
        nonce = os.urandom(12)

        encrypted_content_bytes = aesgcm.encrypt(
            nonce, payload_json_str.encode("utf-8"), None
        )

        # Convert back to string for the packet
        b64_iv = base64.b64encode(nonce).decode("utf-8")
        b64_content = base64.b64encode(encrypted_content_bytes).decode("utf-8")

        # Create empty targets list to populate with recipients
        targets = []

        if recipient == "ALL":
            # If true, send to everyone in our directory
            targets = list(self.user_public_keys.keys())
        else:  # If the message is not being sent to everyone, add the intended recipient to targets
            if recipient in self.user_public_keys:
                targets = [recipient]
            else:
                # If the user was not found, let the user know
                await self.event_queue.put(
                    ("user_not_found_error", f"Error: User {recipient} not found.")
                )
                return

        # If no targets were received, let the user know
        if not targets:
            await self.event_queue.put(("no_targets", "No other users connected."))
            return

        # For all of the nickname(s) in targets.
        # - Get their public RSA key
        # - Encrypt the AES key with the users RSA key
        for target_nick in targets:
            try:
                # Get the user(s) public rsa key
                target_pub_key = self.user_public_keys[target_nick]

                # Encrypt AES Key with users RSA Key
                encrypted_aes_key = target_pub_key.encrypt(
                    aes_key,
                    padding.OAEP(
                        mgf=padding.MGF1(algorithm=hashes.SHA256()),
                        algorithm=hashes.SHA256(),
                        label=None,
                    ),
                )

                # The encrypted aes key, courtesy of rsa. Wicked.
                b64_encrypted_key = base64.b64encode(encrypted_aes_key).decode("utf-8")

                # Build Packet
                message_packet = {
                    "t": "M",
                    "r": target_nick,  # Routing: Individual Nickname
                    # "s": ... server adds this
                    "iv": b64_iv,  # AES Setup
                    "k": b64_encrypted_key,  # The "Key to the Door"
                    "m": b64_content,  # The Message
                }

                await self._send_packet(message_packet)

            except Exception as e:
                logger.error(f"Failed to send to {target_nick}: {e}")

        await self.event_queue.put(
            ("my_message", {"sender": "Me: ", "content": content})
        )

        if recipient == "ALL":
            await self.event_queue.put(
                ("broadcasted_to_users", f"Broadcast sent to {len(targets)} users")
            )
        else:
            await self.event_queue.put(("sent_to_user", f"DM sent to {recipient}"))

    # Generates a list of connected users
    async def display_connected_users(self) -> set[str]:
        # create empty list to store usernames
        online_list = []
        # Go through every username in user_public_keys
        online_list.append(f"Me ({self.nickname})")
        for user in self.user_public_keys:
            # If the current user is not the same as the nickname
            if user != self.nickname:
                online_list.append(user)
            # If the current user is the same as the nickname, add "Me" to the list
        return set(online_list)

    async def _cleanup(self) -> None:
        """
        Properly close connections and clean up resources.
        """
        if self.writer:
            self.writer.close()
            await self.writer.wait_closed()
        self.connected = False


# # --- Execution ---
# if __name__ == "__main__":
#
#     # Take user nickname input before calling asyncio to avoid blocking
#     user_nickname = input("Enter your nickname: ").strip()
#
#     # Make sure that nickname is alphanumeric chars only
#     if user_nickname.isalnum():
#         pass
#     else:
#         logger.error("Invalid characters in nickname. Exiting.")
#         sys.exit(1)
#
#     if len(user_nickname) > MAX_NICKNAME_LENGTH:
#         logger.error(f"Nickname too long. Max length is {MAX_NICKNAME_LENGTH}")
#         sys.exit(1)
#
#     # Check if user_nickname is empty, if it is, kick em out
#     if not user_nickname:
#         logger.error("Nickname cannot be empty.")
#         return.exit(1)
#
#     async def main(host: str, port: int, nickname: str) -> None:
#         # Pass nickname to __init__
#         client = Client(host, port, nickname=user_nickname)
#         await client.start()
#
#     # Run the main asynchronous entry point
#     try:
#         asyncio.run(main("127.0.0.1", 55556, user_nickname))
#     except KeyboardInterrupt as e:
#         logger.info("Client stopped.")
