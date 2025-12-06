from cryptography.hazmat.primitives.asymmetric import rsa, padding
from cryptography.hazmat.primitives import serialization, hashes
from cryptography.hazmat.backends import default_backend
import base64
import getpass
import asyncio
import sys
import json
import datetime


class Client:
    def __init__(self, host: str, port: int, nickname: str) -> None:
        """
        Inits the chat client with RSA key pair and connection settings.

        Prompts user for:
        - Server address and port -- might be a little flimsy, would introduce user error
        - Nickname - same as prev need to rethink this one a bit

        Generates:
        - AES key to encrypt messages
        - 2048-bit RSA key pair for encryption of the AES key
        # Optimization: Hybrid encryption (RSA + AES) used to minimize computational overhead of asymmetric crypto.
        """
        # Initialize client variables and setup basic info.
        # TODO: Security: Input validation for host/port could prevent injection; consider defaults or config files.

        # Aysncio shtuff
        self._reader: None = None
        self.writer: None = None
        # This is so background tasks have a place to put data
        self.event_queue = asyncio.Queue()

        # Store host & port
        self.host = host
        self.port = port

        # Take the nickname input and store it
        self.nickname = nickname

        # Public and private key generation
        self.private_key = rsa.generate_private_key(
            public_exponent=65537, key_size=2048, backend=default_backend()
        )
        self.public_key = self.private_key.public_key()

        # Serialize public key for transmission
        public_key_bytes = self.public_key.public_bytes(
            encoding=serialization.Encoding.PEM,
            format=serialization.PublicFormat.SubjectPublicKeyInfo,
        )
        self.public_key_str = public_key_bytes.decode("utf-8")

        # Store other users' public keys
        self.user_public_keys = {}

        self.connected = False

        self.handshake_packet =
        # packet structure for reference
        # {
        #   "t": "H",              // Type: Handshake
        #   "n": "Alice",          // Nickname (String)
        #   "k": "-----BEGIN..."   // Public Key (PEM String)
        # }
        (
            {  # Set key packet to send public key for message decryption.
                "t": "H",
                "n": self.nickname,
                "k": self.public_key_str,
            }
        )

    async def _receive_packet(self) -> None | dict[str, str]:
        try:
            message: str = await self.reader.readline()
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

    async def _send_packet(self, packet):
        """
        1. Encodes the packet to JSON bytes (with newline).
        2. Writes it to the specific writer.
        3. Drains the writer to ensure it sent.
        Safety: Wrap in try/except to ignore broken pipes.
        """
        try:
            self.writer.write(
                (json.dumps(packet) + "\n").encode("utf-8")
            )
            await self.writer.drain()
        except Exception:  # If an exception occurs
            pass

    @property
    def reader(self):
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
    def reader(self, value):
        self._reader = value

    def store_user_public_key(self, nickname, public_key_str):
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
            print(f"Error storing public key for {nickname}: {e}")

    async def start(self) -> None:

        # Connection
        try:
            await self.connect_to_server(self.host, self.port)
            await self.check_handshake()

            # Bridge between network end ui
            self.event_queue = asyncio.Queue()

        except Exception as e:
            print(f"Failed to connect: {e}")
            return

        try:
            # Listens for messages
            receive_task = asyncio.create_task(self.receive_messages())

            # Launch the tui
            tui_task = asyncio.create_task(self.run_tui())

            # Handles user input
            input_task = asyncio.create_task(self.handle_user_input())

            # Keep the app running
            done, pending = await asyncio.wait(
                [receive_task, tui_task, input_task],
                return_when=asyncio.FIRST_COMPLETED,
            )

            for task in pending:
                task.cancel()

        except Exception:
            return

        finally:
            await self._cleanup()

    async def connect_to_server(self, host, port):
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
            raise Exception("Unable to connect to host and port.")

    # TODO: This currently isn't sending the public keys like it should. It's just checking the nickname
    async def check_handshake(self):
        """
        Async method to check the client handshake response. If successful, the client public key is stored in the Client class object so messages can be sent to and from the connected client(s).

        If no key is received, then the chat is closed as encryption can't be guaranteed.

        Exceptions:
            If an error occurs during the handshake, it will yield an Exception to pass onto the tui.

        yields:
            True/False if the client has been granted encryption and the keys have successfully been exchanged

        i think this would run in O(1) as if there is no message then it will exit immediately. even if data is received, it's being accessed through a json dict which allows for constant time access.

        """

        # Send the Handshake Packet immediately
        self._send_packet(self.handshake_packet)

        # Listen for response (The server might send ERR or DIR)
        # If successful, Server sends nothing specific back immediately,
        # it just proceeds to _synchronise and sends the DIR packet.
        return True

    async def initialise(self):
        """
        a method to initialise the clients connection to the server and to initiate the handshake. This can't be called within the init as that is not async
        """
        await self.connect_to_server(self.host, self.port)
        await self.check_handshake()

    async def receive_messages(self):
        """
        Continuously listens for and yields messages from the server.

        This async method processes incoming messages, parses into json and decrypts messages when necessary

        yields:
            tuple(content, sender) for each message that's been received

        raises:
            silently closes if the connection has been lost

        runs time and space complexity of O(1) , due to constant time and space complexity of each iteration. The number of iterations will depend on the number of messages received, but each message will take constant time and space to process.

        """
        # Continuously listen for messages from the server
        while True:
            packet = await self._receive_packet()

            if not packet:
                break

            # sender = packet.get("r")  # Get recipient
            # content = packet.get("m")  # Get message content

            if packet.get("t") == "M":
                content = self.private_key.decrypt(
                    content.encode( "latin1"),  # The content that is to be decoded and decrypted is encoded into bytes using latin1 encoding,
                    padding.OAEP(
                        mgf=padding.MGF1(algorithm=hashes.SHA256()),  # Padding added
                        algorithm=hashes.SHA256(),
                        label=None,
                    ),
                )
                pass

            if not all([sender, content]):  # confirm that we have valid data
                continue

            yield (content, sender)

    async def handle_user_input(self):
        # Collect input from the user (must be non-blocking)
        # Checks to see if the message is intended for a specific person, if not, broadcast to all. Will need to loop through current connected_users if sending to all.
        # Encrypt the message with the AES key.
        # Encrypt the AES key with the recipients public key. Might be complex loop if sending to more than one user.
        # Build the message packet;
        # {
        #   "t": "M",              // Type: Message
        #   "r": "Bob",            // Recipient: "Nickname" or "ALL"
        #   "s": "Alice",          // Sender (Added by Server)
        #
        #   "iv": "...",           // AES Initialization Vector (Base64 String)
        #   "key": "...",          // The AES Key, encrypted with Recipient's RSA Public Key (Base64 String)
        #   "m": "..."             // The Message Content, encrypted with the AES Key (Base64 String)
        # }
        # Send the packet via _send_packet method
        while True:
            try:
                # Get the input from the user but make it non-blocking
                raw_input = await asyncio.get_event_loop().run_in_executor(None, input)

                # Check for broadcast or DM
                if raw_input.startswith("@"):

                    # Will split nickname from the rest of the message so we can use that to encrypt the message with the intended recipients key.
                    parts = raw_input[1:].split(" ", 1)
                    if len(parts) == 2:
                        recipient = parts[0]
                        content = parts[1]
                    else:
                        print("Usage: @Nickname Message")
                        continue

                    if recipient != "ALL":
                        for user in self.connected

                else:
                    recipient = "ALL"
                    content = raw_input


# def send_messages():
#     # Continuously send messages typed by the user
#     while True:
#         try:
#             message = input("")
#             print(f"Me: {message}") # TODO: remove this line later
#             if message.lower() in {"exit", "quit"}:
#                 print("You have disconnected.")
#                 client.close()
#                 break
#
#             packet = {
#                 "type": "DM",
#                 # "sender": nickname,
#                 "recipient": "ALL",
#                 "timestamp": datetime.datetime.now().strftime("%d-%m-%Y %H:%M"),
#                 "content": message
#             }
#
#             client.send(json.dumps(packet).encode("utf-8"))
#
#         except KeyboardInterrupt:
#             print("\n You have disconnected.")
#             client.close()
#             break
#         except Exception:
#             print("Error sending message.")
#             client.close()
#             break


# --- Execution ---
if __name__ == "__main__":

    # Take user nickname input before calling asyncio to avoid blocking
    user_nickname = input("Enter your nickname: ").strip()

    # Check if user_nickname is empty, if it is, kick em out
    if not user_nickname:
        print("Nickname cannot be empty. Exiting")
        sys.exit(1)

    async def main(host, port, nickname) -> None:
        # Pass nickname to __init__
        client = Client(host, port, nickname=user_nickname)

    # Run the main asynchronous entry point
    try:
        asyncio.run(main("127.0.0.1", 55556, user_nickname))
    except KeyboardInterrupt:
        print("Client stopped.")
