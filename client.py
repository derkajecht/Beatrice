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
    def __init__(self) -> None:
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
        self._reader = None
        self.writer = None

        try:  # Error handling on nickname creation
            self.nickname = self.set_nickname()
        except Exception as e:
            print(str(e))
            sys.exit(1)

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

        self.key_packet = {  # Set key packet to send public key for message decryption.
            "type": "KEY",
            "key": self.public_key_str,
            "sender": self.nickname,
        }

        self.message_packet = {
            # "type": "DM", # TODO: pass type to send_message once built.
            "sender": self.nickname,
            "recipient": "ALL",
            "timestamp": datetime.datetime.now().strftime("%d-%m-%Y %H:%M"),
            "content": "",
        }

        # self.create_socket()
        # self.check_handshake()

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

    def set_nickname(self):
        """
        Gets a user set nickname from the user.

        Raises:
            if nickname is empty, raise exception

        return:
            str nickname

        According to the RAM model, we have to assume that this assignment is being done in constant time - O(1)
        """
        # TODO: verify uniqueness. would need to have the server store the nicknames to be able to check uniqeness - that would be cool
        nickname = input("Set your nickname:  ").strip()
        if nickname == "":
            raise Exception("Invalid nickname, please try again.")
        else:
            return nickname

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
        while True:
            try:
                message = await self.reader.read(
                    1024
                )  # Listen for messages from the server
                if not message:  # if no data received return false
                    break

                message_str = message.decode("utf-8").strip()

            except OSError as e:
                yield ("error", "OSError - {0}".format(e))
                break
            except UnicodeDecodeError as e:
                yield ("error", "UnicodeDecodeError - {0}".format(e))
                break

            # Check server response
            if message_str == "NICK":
                # Send nickname
                try:
                    if self.writer:
                        self.writer.write(self.nickname.encode())
                        await self.writer.drain()
                    yield ("success", "Nickname sent successfully")
                except OSError:
                    # Failed to send nickname
                    yield ("error", "Failed to send nickname")
                    break

            else:
                yield ("error", "Invalid message received from server")
                continue

    async def initialise(self):
        """
        a method to initialise the clients connection to the server and to initiate the handshake. This can't be called within the init as that is not async
        """
        await self.connect_to_server(self.host, self.port)
        await self.check_handshake()

    # def receive_packet(self):
    #     try:
    #         message = await self.reader.read(100)
    #         message_str = message.decode("utf-8")
    #     except (OSError, UnicodeDecodeError):
    #         return False
    #     if not message:
    #         return False
    #     try:
    #         local_packet = json.loads(message_str)
    #         if local_packet.get("type") == "KEY":
    #             self.cipher =
    #         return local_packet
    #     except json.JSONDecodeError:
    #         return message_str

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
            try:
                message = await self.reader.read(
                    1024
                )  # Listen for messages from the server
                if (
                    not message
                ):  # If a message is not received, break from the loop silently
                    break
                message_str = message.decode(
                    "utf-8"
                )  # If message received, decode into UTF-8 format
            except OSError as e:  # If there is an error, break from the loop,
                yield ("error", "OSError - {0}".format(e))
                break
            except asyncio.IncompleteReadError:
                yield ("error", "Incomplete Read Error")
                break
            except UnicodeDecodeError as e:
                continue

            try:
                local_packet = json.loads(
                    message_str
                )  # decrypt the utf8 message into JSON packet if its valid json
            except json.JSONDecodeError as e:  # json error if there is an invalid json
                yield ("error", "JSONDecodeError - {0}".format(e))
                continue

            sender = local_packet.get(
                "sender", "Unknown"
            )  # Assign the sender, message  of a particular packet using get method
            content = local_packet.get("content", "")

            if local_packet.get("type") == "DM":
                content = self.private_key.decrypt(
                    content.encode(
                        "latin1"
                    ),  # The content that is to be decoded and decrypted is encoded into bytes using latin1 encoding,
                    padding.OAEP(
                        mgf=padding.MGF1(algorithm=hashes.SHA256()),  # Padding added
                        algorithm=hashes.SHA256(),
                        label=None,
                    ),
                )
                pass

            if not all([sender, content]):  # validate that we have valid data
                continue

            yield (content, sender)

            # if sender == "SERVER":
            #     yield(f"*** {content} ***")
            # else:
            #     yield(f"[{timestamp}] {sender}: {content}")

    # def send_message(self, message):
    #     if self.connected:


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


if __name__ == "__main__":

    async def main():
        client = Client()
        await client.initialise()

    asyncio.run(main())
