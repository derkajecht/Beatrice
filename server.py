import socket
import threading

HOST = "127.0.0.1"  # Localhost
PORT = 55555  # Choose any open port

server = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
server.bind((HOST, PORT))
server.listen()

clients = []
nicknames = []


def broadcast(message, sender=None):
    # Send a message to all clients except the sender
    for client in clients:
        if client != sender:
            try:
                client.send(message)
            except:
                client.close()
                if client in clients:
                    clients.append(client)


def handle_client(client):
    # Handle incoming messages from a single client
    while True:
        try:
            message = client.recv(1024)
            if not message:
                raise Exception("Client disconnected")
            broadcast(message, sender=client)
        except:
            if client in clients:
                index = clients.index(client)
                nickname = nicknames[index]
                print(f"{nickname} disconnected")
                broadcast(f"{nickname} left the chat!\n".encode("utf-8"))
                clients.remove(client)
                nicknames.remove(nickname)
            client.close()
            break


def receive_connections():
    print(f"Server started on {HOST}:{PORT} — waiting for connections...")
    while True:
        client, address = server.accept()
        print(f"Connected with {address}")

        client.send("NICK".encode("utf-8"))
        nickname = client.recv(1024).decode("utf-8")
        nicknames.append(nickname)
        clients.append(client)

        print(f"{nickname} joined")
        broadcast(f"{nickname} joined the chat!\n".encode("utf-8"))
        client.send("Connected to the server!\n".encode("utf-8"))

        thread = threading.Thread(target=handle_client, args=(client,))
        thread.start()


if __name__ == "__main__":
    receive_connections()
