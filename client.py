import socket
import threading
import sys
import json
import datetime

HOST = "127.0.0.1"
PORT = 55556

client = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
try:
    client.connect((HOST, PORT))
except Exception as e:
    print(f"Could not connect to server: {e}")
    sys.exit()


def receive_messages():
    # Continuously listen for messages from the server
    packet = {
        "type": "DM",
        "sender": nickname,
        "recipient": "ALL",
        "timestamp": datetime.datetime.now().strftime("%d-%m-%Y %H:%M"),
        "content": ""
    }
    while True:
        try:
            message = client.recv(1024)
            if not message:
                break

            message_str = message.decode("utf-8")

            if message_str == "NICK":
                client.send(nickname.encode("utf-8"))

            elif message_str == "Connected to the server!\n":
                print(f"*** {message_str.strip()} ***")

            else:
                try:
                    packet = json.loads(message_str)

                    sender = packet.get("sender", "Unknown")
                    content = packet.get("content", "")
                    timestamp = packet.get("timestamp", "")

                    if sender == "SERVER":
                        print(f"*** {content} ***")
                    else:
                        print(f"[{timestamp}] {sender}: {content}")

                except json.JSONDecodeError:
                    print(f"[Raw message]: {message_str}")

        except Exception:
            print("Disconnected from server.")
            client.close()
            break



def send_messages():
    # Continuously send messages typed by the user
    while True:
        try:
            message = input("")
            print(f"Me: {message}")
            if message.lower() in {"exit", "quit"}:
                print("You have disconnected.")
                client.close()
                break

            packet = {
                "type": "DM",
                "sender": nickname,
                "recipient": "ALL",
                "timestamp": datetime.datetime.now().strftime("%d-%m-%Y %H:%M"),
                "content": message
            }

            client.send(json.dumps(packet).encode("utf-8"))

        except KeyboardInterrupt:
            print("\n You have disconnected.")
            client.close()
            break
        except Exception:
            print("Error sending message.")
            client.close()
            break


if __name__ == "__main__":
    nickname = input("Choose your nickname: ")
    # Start threads
    threading.Thread(target=receive_messages, daemon=True).start()
    send_messages()
