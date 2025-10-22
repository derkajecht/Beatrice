import socket
import threading
import sys

HOST = "127.0.0.1"
PORT = 55555

nickname = input("Choose your nickname: ")

client = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
try:
    client.connect((HOST, PORT))
except Exception as e:
    print(f"Could not connect to server: {e}")
    sys.exit()


def receive_messages():
    # Continuously listen for messages from the server
    while True:
        try:
            message = client.recv(1024).decode("utf-8")
            if message == "NICK":
                client.send(nickname.encode("utf-8"))
            elif message:
                print(message)
            else:
                print("Connection closed by server.")
                client.close()
                break
        except Exception:
            print("Disconnected from server.")
            client.close()
            break


def send_messages():
    # Continuously send messages typed by the user
    while True:
        try:
            message = input("")
            if message.lower() in {"exit", "quit"}:
                client.close()
                print("You have disconnected.")
                break
            full_message = f"{nickname}: {message}"
            client.send(full_message.encode("utf-8"))

            # See your own message in the chat window
            print(f"Me: {message}")
        except KeyboardInterrupt:
            client.close()
            print("\nDisconnected.")
            break
        except Exception:
            print("Error sending message.")
            client.close()
            break


# Start threads
threading.Thread(target=receive_messages, daemon=True).start()
send_messages()
