import asyncio
import base64
import hashlib
import json
import logging
import os
import secrets
import sys
from collections import deque

import httpx
import websockets
from cryptography.hazmat.backends import default_backend
from cryptography.hazmat.primitives import hashes, serialization
# Cryptography imports
from cryptography.hazmat.primitives.asymmetric import padding, rsa
from cryptography.hazmat.primitives.asymmetric.rsa import (RSAPrivateKey,
                                                           RSAPublicKey)
from cryptography.hazmat.primitives.ciphers.aead import AESGCM

from .crypto_utils import check_or_create_keys, get_public_key_bytes

# Logger setup
logger = logging.getLogger(__name__)
logging.basicConfig(
   filemode="w", level=logging.INFO, format="%(asctime)s - %(levelname)s - %(message)s"
)

# Constants
MAX_PACKET_LENGTH = 256 * 1024  # 256kb (should be large enough)
RSA_KEY_SIZE = 2048
AES_KEY_SIZE = 256
TARGET_PAYLOAD_SIZE = 4096

class Client:
    def __init__(self, host: str, port: int, nickname: str):
        # Initialize client properties
        self.host: str = host
        self.port: int = port

        # Set nickname
        self.nickname: str = nickname

        # Queue for sending events to the UI
        self.event_queue: asyncio.Queue = asyncio.Queue(maxsize=100)

        # Init connected status as false since we are not connected yet
        self.connected: bool = False

        # Nonce set for checking seen nonces
        self.seen_nonces = deque(maxlen=2000)

        # Key pair generation
        self.public_key, self.private_key = check_or_create_keys()

        # Serialise public key
        self.public_key_str = get_public_key_bytes(self.public_key)

        # Store other users public keys PEM format
        self.user_public_keys: dict[str, str] = {}

        # cache deserialized keys for faster signing
        self._key_cache: dict[str, str] = {}

        # /--- Init handshake packet data
        self.handshake_packet: dict[str, str] = {
            "t": "H",
            "n": self.nickname,
            "k": self.public_key_str,
        }

    def get_public_key(self, nickname: str):
        if nickname not in self._key_cache:
            pem = self.user_public_keys[nickname]
            self._key_cache[nickname] = serialization.load_pem_public_key(pem)
        return self._key_cache[nickname]

    async def login(self):
        url = f"http://{self.host}:{self.port}/login"

        # Create payload to send to server
        payload = {
            "t": "H",
            "n": self.nickname,
            "k": self.public_key_str
        }

        async with httpx.AsyncClient() as client:
            try:
                response = await client.post(url, json=payload)
                # Check status codes from the server;
                # 200 - Success, pass user to websocket connection
                if response.status_code != 200:
                    logger.error(f"Login failed with status: {response.status_code}")
                    await client.aclose()
                    return
                await self.connect_to_server()
                logger.info("Handshake successful. Starting websocket.")
            except httpx.HTTPStatusError as e:
                logger.error(f"Request error: {e} {e.response.status_code}")


    async def connect_to_server(self):
        """Connect to the server using WebSocket."""
        uri = f"ws://{self.host}:{self.port}/ws/{self.nickname}"
        # Connect to server via websocket. Server listening on /ws/{nickname}
        self.websocket = await websockets.connect(uri)
        if self.websocket:
            # If successful, set connected to True
            self.connected = True

    async def listen(self):
        while self.connected:
            packet = await self._receive_packet()
            if not packet:
                break

            # --- DIR PACKET ---
            if packet.get("t") == "DIR":
                for user_data in packet.get("p", []):
                    nickname = user_data.get("n")
                    public_key = user_data.get("k")
                    if nickname and public_key:
                        self.user_public_keys[nickname] = public_key
            # --- JOIN PACKET ---
            elif packet.get("t") == "J":
                # Extract the new nickname and key from the packet
                new_nick, new_key = packet.get("n"), packet.get("k")

                if new_nick and new_key:
                    self.user_public_keys[new_nick] = new_key

                    # update the tui with the name of the new user
                    await self.event_queue.put(
                        ("join_packet", f"{new_nick} has joined the chat!")
                    )
            # --- LEAVE PACKET ---
            elif packet == "L":
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
            elif packet == "ERR":
                error_message = packet.get("c")
                # update the tui
                await self.event_queue.put(("err", error_message))
            # --- MESSAGE PACKET ---
            elif packet.get("t") == "M":
                await self.receive_messages(packet)

    async def disconnect(self):
        if not self.websocket:
            logger.warning("No active websocket to disconnect from.")
            self.connected = False
            return
        # Create leave packet
        leave_packet = {
            "t": "L",
            "n": self.nickname
        }
        try:
            await self.websocket.send(json.dumps(leave_packet))
            logger.info("Leave packet sent to server.")
            await self.websocket.close()
            logger.warning("WebSocket connection closed.")
            self.websocket = None
            self.connected = False
        except websockets.ConnectionClosed as e:
            logger.error(f"Error closing websocket: {e}")
            self.websocket = None
            self.connected = False
        except Exception as e:
            logger.error(f"Error during disconnect: {e}")
            self.websocket = None
            self.connected = False

    async def _send_packet(self, packet: dict):
        """
        helper method to avoid re-writing the same lines of code over and over
        """
        if not self.websocket:
            logger.warning("No active websocket to receive from.")
            self.connected = False
            return
        try:
            await self.websocket.send(json.dumps(packet))
        except websockets.exceptions.ConnectionClosed:
            logger.error("Failed to send. Connection closed.")
            self.connected = False

    async def _receive_packet(self) -> dict | None:
        """
        helper method to avoid re-writing the same lines of code over and over
        """
        try:
            if not self.websocket or not self.connected:
                return None
            message = await self.websocket.recv()
            packet = json.loads(message)
            return packet
        except websockets.exceptions.ConnectionClosed:
            logger.info("Connection closed.")
            self.connected = False
            return None
        except json.JSONDecodeError as e:
            logger.error(f"Invalid JSON: {e}")
            self.connected = False
            return None
        except Exception as e:
            logger.error(f"Invalid JSON: {e}")

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

    async def receive_messages(self, packet: dict) -> None:
        """
        Continuously listens for packets from the server and handles them.

        Processes different packet types: messages, errors, directory updates, etc.
        """
        # --- MESSAGE PACKET ---
        # Handle message
        recipient = packet.get("r")
        sender = packet.get("s")
        encrypted_msg_b64 = packet.get("m")
        encrypted_key_b64 = packet.get("k")
        iv_b64 = packet.get("iv")
        signature_b64 = packet.get("h")
        # check to see if all parts of the message packet have been received, if not, chuck em out and log an error
        if not all(
            [
                sender,
                encrypted_msg_b64,
                encrypted_key_b64,
                iv_b64,
                signature_b64,
            ]
        ):
            logger.error("Message packet incomplete")
        final_packet = {
                "s": sender,
                "encrypted_message_b64": encrypted_key_b64,
                "encrypted_key_b64": encrypted_key_b64,
                "iv_b64": iv_b64,
                "signature_b64": signature_b64,
        }
        await self.decrypt_message(final_packet)

    async def decrypt_message(self, message_packet):
        try:
            # Decode from base64
            encrypted_key = base64.b64decode(encrypted_key_b64)
            encrypted_message = base64.b64decode(encrypted_msg_b64)
            iv = base64.b64decode(iv_b64)
            signature = base64.b64decode(signature_b64)

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

            # Create local hash of the content to compare against the hash received
            digest = hashes.Hash(hashes.SHA256(), backend=default_backend())
            digest.update(decrypted_content_bytes)
            payload_hash = digest.finalize()

            # Get senders public key to use to verify the signature
            sender_public_key = self.user_public_keys.get(sender)

            if sender_public_key:
                try:
                    sender_public_key.verify(
                        signature,
                        payload_hash,
                        padding.PSS(
                            mgf=padding.MGF1(hashes.SHA256()),
                            salt_length=padding.PSS.MAX_LENGTH,
                        ),
                        hashes.SHA256(),
                    )
                except Exception as e:
                    logger.error(
                        f"SECURITY: Invalid signature from {sender}. Message rejected. {e}"
                    )
            logger.error(f"No public key for {sender}")

            # decrypt the json content using utf-8 and json.load
            decrypted_json_str = decrypted_content_bytes.decode("utf-8")
            message_payload = json.loads(decrypted_json_str)

            # Check for replay attacks. If the nonce has been seen before, let the user know
            replay_nonce = message_payload.get("nonce")
            if replay_nonce in self.seen_nonces:
                logger.warning(
                    f"SECURITY WARNING: Replay attack detected! Dropping packet {replay_nonce}"
                )

            # If we get here, the message is unique and not been replayed, so add it to the set
            self.seen_nonces.append(replay_nonce)

            # Send to tui
            await self.event_queue.put(("message", message_payload))

        # Skip invalid messages
        except Exception as e:
            logger.error(f"Error decrypting message: {e}")

    async def send_message(self, content: str):
        """
        Collect input from the user. Checks if the message is intended for a specific person, if not, broadcast to all. Remove whitespace at the start and end of each message. Encrypt the message with AES, then encrypt the AES key with RSA public key. Build the message packet, and send via the _send_packet helper method. bosh.
        """

        # Packet structure for reference;
        # {
        #   "t": "M",              // Type: Message
        #   "r": "Bob",            // Recipient: "Nickname" or "ALL"
        #   "s": "Alice",          // Sender (Added by Server)
        #   "iv": "...",           // AES Initialization Vector (Base64 String)
        #   "k": "...",            // The AES Key, encrypted with Recipient's RSA Public Key (Base64 String)
        #   "m": "..."             // The Message Content (sender, message content, nonce), encrypted with the AES Key (Base64 String)
        #   "h": "..."             // Message digest, to protect against tampering whilst in transit
        # }

        # if the user enters quit or exit, stop the program
        if content.strip().lower() in ["exit", "quit", ":q"]:
            sys.exit(0)

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
                        "Cannot send a direct message to yourself. Please try again."
                    )
                    return

                # Update the content varible with the message content
                content = f"@{parts[1].strip()}"

            else:  # If message doesn't follow the correct syntax let the user know
                await self.event_queue.put(
                    ("dm_usage_error", "Usage: @Nickname Message")
                )
                return
        else:
            pass

        # Generate replay_nonce for stopping replay mitm attacks
        replay_nonce = secrets.token_hex(16)

        # Create payload dict
        payload_dict = {
            "sender": self.nickname,
            "content": content,
            "nonce": replay_nonce,
        }

        # Convert dictionary into JSON string
        payload_packet = json.dumps(payload_dict)

        # Get length of payload to calculate the remaining bytes for padding
        payload_length = len(payload_packet.encode("utf-8"))
        # Calculate how much padding is needed
        padding_needed = TARGET_PAYLOAD_SIZE - payload_length - 50

        # If needed, generate random junk data to pad the packet to the target size
        if padding_needed > 0:
            junk_data = secrets.token_hex(padding_needed // 2)
            payload_dict["_pad"] = junk_data

        # Serialize the packet dictionary to a JSON string
        payload_bytes = payload_packet.encode("utf-8")

        # SHA256 hash of the base64 string, to be added to the packet for verification
        digest = hashes.Hash(hashes.SHA256(), backend=default_backend())
        digest.update(payload_bytes)
        payload_hash = digest.finalize()

        # Signature created with senders private key
        signature = self.private_key.sign(
            payload_hash,
            padding.PSS(
                mgf=padding.MGF1(hashes.SHA256()), salt_length=padding.PSS.MAX_LENGTH
            ),
            hashes.SHA256(),
        )

        # Generate AES and IV for encryption
        aes_key = AESGCM.generate_key(bit_length=AES_KEY_SIZE)
        aesgcm = AESGCM(aes_key)
        iv_nonce = os.urandom(12)

        encrypted_content_bytes = aesgcm.encrypt(iv_nonce, payload_bytes, None)

        # Convert back to string for the packet
        b64_iv = base64.b64encode(iv_nonce).decode("utf-8").replace("\n", "")
        b64_content = (
            base64.b64encode(encrypted_content_bytes).decode("utf-8").replace("\n", "")
        )
        b64_signature = base64.b64encode(signature).decode("utf-8").replace("\n", "")

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

        # NOTE: Optimization needed. Currently, we encrypt and upload the full message body N times for N users. A better approach would be to encrypt the body once (AES), and only encrypt the AES key N times (RSA), sending a single payload to the server.

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
                    "k": b64_encrypted_key,  # The key to the door
                    "m": b64_content,  # The message
                    "h": b64_signature,
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
    async def display_connected_users(self):
        # Go through every username in user_public_keys
        return set(self.user_public_keys.keys())
