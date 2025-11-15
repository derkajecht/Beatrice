import getpass
import socket
import threading
import sys
import json
import datetime
from cryptography.fernet import Fernet

class Client:
    def __init__(self) -> None:
        self.host = "127.0.0.1"
        self.port = 55556
        self.key = Fernet.generate_key()
        self.cipher = Fernet(self.key)
        self.nickname = self.set_nickname()
        self.key_packet = {
                "type": "KEY",
                "key": self.key.decode("latin1"),
                "sender": self.nickname
        }
        self.client = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self.connected = False
        self.packet = {
        # "type": "DM", # TODO: pass type to send_message once built.
        "sender": self.nickname,
        "recipient": "ALL",
        "timestamp": datetime.datetime.now().strftime("%d-%m-%Y %H:%M"),
        "content": ""
        }
        self.create_socket()
        self.check_handshake()

    def create_socket(self):
        # Method for creating IPV4 TCP socket connection
        yield("Starting up network...")
        try:
            self.client.connect((self.host, self.port))
        except (OSError):
            yield(f"Could not connect to server")
            sys.exit()

    def set_nickname(self):
        # Method for user to set a nickname. TODO: verify uniqueness.
        nickname = input("Set your nickname:  ").strip()
        if nickname == "":
            yield("Invalid nickname, please try again.")
            self.set_nickname()
        else:
            self.nickname = nickname

    def check_handshake(self):
        # Method to check server handshake response.
        # If server sends correct handshake response then client will enter main chat loop.
        try:
            message = self.client.recv(1024)
            message_str = message.decode("utf-8")
        except (OSError, UnicodeDecodeError):
            return False
        if not message:
            return False
        if message_str == "NICK":
            try:
                self.client.send(self.nickname.encode())
                return True
            except (OSError):
                yield("Failed to send nickname. Connection may have been lost. Please try reconnecting.")
                return False

        elif message_str == "Connected to the server!\n":
            self.connected = True
            self.client.send(json.dumps(self.key_packet).encode())
            return True
        else:
            yield("Invalid handshake")
            return False

    def receive_packet(self):
        try:
            message = self.client.recv(1024)
            message_str = message.decode("utf-8")
        except (OSError, UnicodeDecodeError):
            return False
        if not message:
            return False
        try:
            local_packet = json.loads(message_str)
            if local_packet.get("type") == "KEY":
                self.cipher = Fernet(local_packet["key"].encode("latin1"))
            return local_packet
        except json.JSONDecodeError:
            return message_str

    def receive_messages(self):
        # Continuously listen for messages from the server
        while True:
            try:
                message = self.client.recv(1024)
                message_str = message.decode("utf-8")
            except (OSError, UnicodeDecodeError):
                return False

            if not message:
                break

            try:
                local_packet = json.loads(message_str)

                sender = local_packet.get("sender", "Unknown")
                content = local_packet.get("content", "")
                timestamp = local_packet.get("timestamp", "")

                try:
                    if local_packet.get("type") == "DM":
                        content = self.cipher.decrypt(content.encode("latin1")).decode("utf-8")
                except cryptography.fernet.InvalidToken:

                    return False
            except json.JSONDecodeError:
                return message_str

            if not all([message_str, sender, content]):
                continue

            if sender == "SERVER":
                yield(f"*** {content} ***")
            else:
                yield(f"[{timestamp}] {sender}: {content}")


    def send_message(self, message):
        if self.connected:




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
    client = Client()
    # Start threads
    threading.Thread(target=client.receive_messages, daemon=True).start()
    send_messages()
