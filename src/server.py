import argparse
from datetime import datetime
from cachetools import TTLCache, cached
import logging
import os
import sqlite3
import aiosqlite
import sys
import secrets
from contextlib import asynccontextmanager

import uvicorn
from cryptography.hazmat.primitives import serialization
from fastapi import (FastAPI, HTTPException, WebSocket, WebSocketDisconnect,
                     WebSocketException)
from pydantic import BaseModel, Field

# Logger setup
logger = logging.getLogger(__name__)
logging.basicConfig(
    filename="beatrice_server.log", level=logging.INFO, format="%(asctime)s - %(levelname)s - %(message)s"
)

# Constants
# NICKNAME_SUFFIX_MIN = 100
# NICKNAME_SUFFIX_MAX = 999


@asynccontextmanager
async def init_db(app: FastAPI):
    """
    Open connection to the db, create the users and messages tables
    if it doesnt exist. Close it on server shutdown
    """
    if sys.platform == "win32":
        # Windows
        database_path = os.path.expandvars(
            r"%APPDATA%\beatrice\beatrice_server.db")
    elif sys.platform == "darwin":
        # macOS
        database_path = os.path.expanduser(
            "~/Library/Application Support/beatrice/beatrice_server.db")
    else:
        # Linux and other Unix
        database_path = os.path.expanduser(
            "~/.config/beatrice/beatrice_server.db")

    try:
        os.makedirs(os.path.dirname(database_path), exist_ok=True)

        con = sqlite3.connect(database_path)
        cur = con.cursor()
        cur.execute(
            """
            CREATE TABLE IF NOT EXISTS users (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            nickname TEXT NOT NULL UNIQUE ,
            public_key TEXT NOT NULL,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
            )
            """
        )
        cur.execute(
            """
            CREATE TABLE IF NOT EXISTS messages (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            sender TEXT NOT NULL, -- "ALL" for broadcasts, or nickname for dms
            recipient TEXT NOT NULL,
            iv TEXT NOT NULL,
            encrypted_key TEXT NOT NULL,
            encrypted_content TEXT NOT NULL,
            signature TEXT NOT NULL,
            timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY (sender) REFERENCES users(nickname)
            )
            """
        )
        con.commit()
        app.state.db_path = database_path
        con.close()
        logger.info("Database initialised at {}".format(database_path))
        yield
    except Exception as e:
        logger.error("Database initialisation failed: {}".format(e))
        raise


class ConnectionManager:
    """ Handles client connect and disconnect. """

    def __init__(self):
        # Dict of current active users - "jordan": {"ws": 1234, "pk":
        # "-----BEGIN PUBLIC KEY..."}
        self.active_connections: dict[str, dict] = {}

        # Dict of pending connections once the hand shake is confirmed, this will temporarily store
        # the public key to be passed into the websocket connection method
        self.pending_connections: dict[str, str] = {}

    async def connect(self, nickname, websocket, public_key):
        """ Start websocket and add user to active connections. """
        try:
            # Accept websocket connection
            await websocket.accept()
            # Add information to the dict to store user connection data
            self.active_connections[nickname] = {
                "ws": websocket, "pk": public_key}
            logger.info("{} joined the server".format(nickname))
        except WebSocketException as e:
            logger.error("Error connecting to {nickname}: {}".format(e))
            raise

    async def disconnect(self, nickname):
        """ Remove user from active connections. """
        # Get websocket of user and close it
        ws = self.active_connections[nickname]["ws"]
        pk = self.active_connections[nickname]["pk"]
        if ws:
            logger.info("{} left the server".format(nickname))
            leave_packet = {"t": "L", "n": nickname, "k": pk}
            await manager.broadcast_message(leave_packet, nickname)
            del self.active_connections[nickname]
            await ws.close()

    async def broadcast_message(self, message_payload: dict, exclude=None):
        """ Send message to all connected users. """
        for nickname, data in self.active_connections.items():
            if nickname != exclude:
                await data["ws"].send_json(message_payload)
        return True

    async def dm_message(self, message_payload, target):
        """ Send message to specific user. """
        if target in self.active_connections:
            dm_target = self.active_connections[target]["ws"]
            await dm_target.send_json(message_payload)
            return True
        return False


# Init fastapi
app = FastAPI(lifespan=init_db)
# Init connection manager class
manager = ConnectionManager()


class HandshakePacket(BaseModel):
    """Defines the handshake packet structure and checks validity"""
    packet_type: str = Field(alias="t")
    nickname: str = Field(alias="n")
    public_key: str = Field(alias="k")

    # Check validity of packet
    def is_valid(self) -> bool:
        return self.packet_type == "H" and len(self.nickname) > 0 and len(
            self.public_key) > 0  # Not sure if the beginswith is necessary

# --- REGISTER USER ---
@app.post("/login")
async def authenticate_user(packet: HandshakePacket):
    # Checks if user exists, if not pass to create_user method to add to db
    # If existing user, proceed to create websocket connection
    if not packet.is_valid():
        logger.error("Invalid packet received. Please try again.")
        raise HTTPException(status_code=400, detail="Invalid handshake")

    # Extract data from the packet
    nickname = packet.nickname
    public_key = packet.public_key

    # Stop duplicate logins
    if nickname in manager.active_connections:
        raise HTTPException(
            status_code=409,
            detail="User already connected. Please disconnect first.")

    # Check if user in users db
    async with aiosqlite.connect(app.state.db_path) as db:
        cur = await db.cursor()
        await cur.execute(
            "SELECT nickname, public_key FROM users WHERE nickname=?", (nickname,))
        user = await cur.fetchone()
        try:
            # check if user already in db
            if user:
                nick, pub_key = user
                is_new = False
                if public_key and public_key != pub_key:
                    logger.error("Public key from database does not match.")
                    raise HTTPException(
                        status_code=400, detail="Public key mismatch.")
            else:
                nick = nickname
                pub_key = public_key
                is_new = True
                # If this passes, the public key is valid
                try:
                    serialization.load_pem_public_key(pub_key.encode("utf-8"))
                except Exception as e:
                    logger.error("Invalid public key {}".format(e))
                    raise HTTPException(
                        status_code=400, detail="Invalid public key")

            # create nonce to send to client for challenge response type handshake
            pending_auth = secrets.token_hex(16)

            challenge_packet = {
                    "n": nick,
                    "c": pending_auth,
                    "pk": pub_key,
                    "is_new": is_new
                    }

            await manager.send_json(challenge_packet)

        except Exception as e:
            logger.error("Error: {}".format(e))

# TODO: need a way for users to prove they own the key they are using to connect
# @app.post("/verify")
# def verify_user(challenge_packet: dict):
#     nickname = challenge_packet["s"]
#     signature = challenge_packet["sig"]
#     manager.pending_connections[nickname] = public_key
#
#     return {
#             "status": "success",
#             "message": f"{nickname} added successfully"
#             }
# elif user:
#     # Validate stored key to provided key
#                     _, stored_public_key = user
#                     if stored_public_key != public_key:
#                         raise HTTPException(
#                                 status_code=403, detail="Public key mismatch")
#                     logger.info(
#                             "Public key verified successfully. Proceeding with request.")
#                     return {
#                             "status": "success",
#                             "message": "Public key verification successful."
#                             }
#
#                 # Add user to db
#                 await cur.execute(
#                     "INSERT INTO users (nickname, public_key) VALUES (?, ?)",
#                     (nickname,
#                      public_key))
#                 await db.commit()
#                 logger.info("{} added successfully.".format(nickname))

@app.websocket("/ws/{nickname}")
async def beatrice(websocket: WebSocket, nickname: str):
    try:
        public_key = manager.pending_connections.pop(nickname)
    except KeyError:
        logger.error("{} not found. Please try reconnecting.".format(nickname))
        raise
    try:
        # Make connection via websocket
        await manager.connect(nickname, websocket, public_key)
    except Exception:
        raise

    # Send dir packet
    # Get current active user list
    current_users = [{"n": n, "k": d["pk"]}
                     for n, d in manager.active_connections.items() if n != nickname]
    # create dir packet
    dir_packet = {"t": "DIR", "p": current_users}
    await websocket.send_json(dir_packet)

    # Send join packet
    join_packet = {"t": "J", "n": nickname, "k": public_key}
    await manager.broadcast_message(join_packet, exclude=nickname)

    # MESSAGE LOOP
    while True:  # continuously listen for incoming messages
        try:
            message_packet = await websocket.receive_json()

            packet_type = message_packet.get("t")
            recipient = message_packet.get("r")
            sender = message_packet.get("s")
            iv = message_packet.get("iv")
            encrypted_key = message_packet.get("k")
            encrypted_content = message_packet.get("m")
            signature = message_packet.get("h")

            if packet_type == "M":
                if not all([packet_type, recipient, sender, iv,
                           encrypted_key, encrypted_content]):
                    logger.error(
                        "Invalid message packet sent from {} to {} received. Missing packet fields.".format(
                            sender, recipient))
                    continue
                # DM LOGIC ---
                if recipient != "ALL":
                    if recipient not in manager.active_connections:
                        logger.warning("{} not found.".format(recipient))
                        continue

                    # push message packet to db for persistant storage
                    async with aiosqlite.connect(app.state.db_path) as db:
                        cur = await db.cursor()
                    try:
                        await cur.execute("""
                        INSERT INTO messages (recipient, sender, iv, encrypted_key, encrypted_content, signature)
                        VALUES (?, ?, ?, ?, ?, ?)

                        """, (
                            recipient,
                            sender,
                            iv,
                            encrypted_key,
                            encrypted_content,
                            signature
                        )
                        )
                        await db.commit()
                        await db.close()
                    except Exception as e:
                        logger.error("Error occurred while writing to the database: {}".format(e))

                    success = await manager.dm_message(message_packet, recipient)
                    if not success:
                        err_packet = {
                            "t": "ERR", "c": f"Error sending message to {recipient}"}
                        await websocket.send_json(err_packet)
                else:
                    recipients = [
                        n for n in manager.active_connections if n != nickname]
                    async with aiosqlite.connect(app.state.db_path) as db:
                        cur = await db.cursor()
                    try:
                        await cur.execute("BEGIN TRANSACTION")
                        for target in recipients:
                            await cur.execute("""
                            INSERT INTO messages (recipient, sender, iv, encrypted_key, encrypted_content, signature)
                            VALUES (?, ?, ?, ?, ?, ?)

                            """, (
                                target,
                                sender,
                                iv,
                                encrypted_key,
                                encrypted_content,
                                signature
                            )
                            )
                        await db.commit()
                    except Exception as e:
                        await db.rollback()
                        err_packet = {
                            "t": "ERR", "c": "Error occurred while writing to the database"}
                        logger.error("{}: {}".format(err_packet, e))
                        await websocket.send_json(err_packet)
                    finally:
                        await db.close()

                    success = await manager.broadcast_message(message_packet, exclude=nickname)
                    if not success:
                        err_packet = {
                            "t": "ERR", "c": "Error broadcasting message"}
                        await websocket.send_json(err_packet)
            if packet_type != "M":
                logger.warning("Unexpected packet '{}' from {}".format(packet_type, recipient))
                continue

        except WebSocketDisconnect:
            logger.error("{} disconnected normally.".format(nickname))
            break
        except WebSocketException as e:
            logger.error("WebSocket error: {}".format(e))
            break
    await manager.disconnect(nickname)


# TODO: Finish writing this, stores messages inside sqlite db
# async def persistant_messages(packet):
#     """Add message packets to the db. Store the encrypted blob, sender and recipient"""

# --- Execution ---
if __name__ == "__main__":

    parser = argparse.ArgumentParser(
        prog="Beatrice Server",
        description="Start a Beatrice server",
        add_help=False,
        conflict_handler="resolve",
    )
    parser.add_argument(
        "--host",
        type=str,
        default="0.0.0.0",
        help="IP to bind the server to (default: '0.0.0.0')",
    )
    parser.add_argument(
        "--port",
        type=int,
        default=55556,
        help="Port to bind the server to (default: 55556)",
    )

    parser.add_argument(
        "-h",
        "--help",
        action="help",
        default=argparse.SUPPRESS,
        help="Show this help message",
    )

    args = parser.parse_args()

    # Run the main async entry point
    try:
        # asyncio.run(server.start_server(args.host, args.port))
        uvicorn.run(app, host=args.host, port=args.port)
    except KeyboardInterrupt:
        logger.info("Server stopped.")
        print("Server stopped.")
