import asyncio
import threading
import json

HOST = "127.0.0.1"  # Localhost
PORT = 55556  # Choose any open port

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
                client.send(json.dumps(message).encode("utf-8"))
            except:
                client.close()
                if client in clients:
                    clients.append(client) # FIX: is this not supposed to be remove the client? its adding again


def handle_client(client):
    # Handle incoming messages from a single client
    while True:
        try:
            message = client.recv(1024)
            json_message = json.loads(message.decode("utf-8"))
            if not json_message:
                raise Exception("Client disconnected")
            broadcast(json_message, sender=client)
        except:
            if client in clients:
                index = clients.index(client)
                nickname = nicknames[index]

                leave_packet = {
                    "type": "SERVER_INFO",
                    "sender": "SERVER",
                    "recipient": "ALL",
                    "timestamp": "...",
                    "content": f"{nickname} has left the chat!"
                }

                broadcast(leave_packet)
                clients.remove(client)
                nicknames.remove(nickname)
            client.close()
            break


def receive_connections():
    print(f"Server started on {HOST}:{PORT} — waiting for connections...")
    join_packet = {
        "type": "SERVER_INFO",
        "sender": "SERVER",
        "recipient": "ALL",
        "timestamp": "...",
        "content": f"has joined the chat!"
    }
    while True:
        client, address = server.accept()
        print(f"Connected with {address}")
        client.send("NICK".encode("utf-8")) # FIX: change handshake message/method
        nickname = client.recv(1024).decode("utf-8")
        nicknames.append(nickname)
        clients.append(client)
        broadcast(join_packet)

        thread = threading.Thread(target=handle_client, args=(client,))
        thread.start()


if __name__ == "__main__":
    receive_connections()
