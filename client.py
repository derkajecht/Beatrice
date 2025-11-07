import socket
import threading
import sys


client = socket.socket(socket.AF_INET, socket.SOCK_STREAM)

nickname = input("Choose your nickname: ")


# Find the first open port in the given range
def find_open_port(start_port=1, end_port=8081):
    for port in range(start_port, end_port):
        with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
            sock.settimeout(0.1)
            res = sock.connect_ex(("localhost", port))
            if res == 0:
                return port
    return None


# Set localhost & the available port (as identified above)
HOST = "localhost"
PORT = find_open_port()

# If no port is available, exit
if PORT is None:
    print("No open ports available.")
    sys.exit(1)

# Attempt connection
try:
    client.connect((HOST, PORT))
except Exception as e:
    print(f"Could not connect to server: {e}")
    sys.exit()


# Continuously listen for messages from the server
def receive_messages():
    while True:
        try:
            message = client.recv(1024).decode("utf-8")
            if message == "whoami":
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


# Continuously send messages typed by the user
def send_messages():
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
