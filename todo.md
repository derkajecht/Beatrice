# TUI Chat App: Refactoring Roadmap

## Phase 1: Server Logic (The "BeatriceServer" Class)
*Goal: Finalize the implementation of the server class structure designed in the refactor.*

- [ ] **Method 1: `start_server`**
    - [ ] Initialize `asyncio.start_server` with `self.handle_client`.
    - [ ] **Logic:** Use `await server.serve_forever()` (Keep running continuously).

- [ ] **Method 2: `handle_client(self, reader, writer)`**
    - [ ] Update signature to accept `reader` and `writer`.
    - [ ] **Manager Logic:** Call `_handshake` -> if success, Call `_synchronise` -> Call `_message_loop` -> finally Call `_cleanup`.

- [ ] **Method 3: `_handshake(self, reader, writer)`**
    - [ ] **Input:** Use `await asyncio.wait_for(reader.readline(), timeout=5.0)` (Security Timeout).
    - [ ] **Validation:** Check if JSON contains "t": "H", "n": Nickname, and "k": Valid PEM Key.
    - [ ] **Uniqueness:** Check if Nickname exists. If yes, append random ID (e.g., "Bob#44").
    - [ ] **Storage:** Save `{ "writer": writer, "key": key }` into `self.connected_users`.
    - [ ] **Return:** Return the **Final Nickname** string to the caller.

- [ ] **Method 4: `_synchronise(self, writer, nickname)`**
    - [ ] **Action A:** Send the new user a list of *all other* connected users + keys (`t`: "DIR").
    - [ ] **Action B:** Loop through `connected_users` and send a "User Joined" packet (`t`: "J", "n": Nickname, "k": Key) to everyone else.

- [ ] **Method 5: `_message_loop(self, reader, sender_nickname)`**
    - [ ] **Loop:** `while True`, `await reader.readline()`.
    - [ ] **Routing:** Parse JSON. If `r` == "ALL", broadcast. If `r` == "Target", look up Target's writer and send.
    - [ ] **Framing:** Ensure every outgoing write includes `+ b'\n'`.

- [ ] **Method 6: `_cleanup(self, nickname, writer)`**
    - [ ] Close the writer.
    - [ ] Remove `nickname` from `self.connected_users`.
    - [ ] Broadcast "User Left" packet (`t`: "L") to remaining users.

---

## Phase 2: Client Refactor (Matching the Protocol)
*Goal: Update the Client to speak the same language (Newlines & JSON Structure) as the new Server.*

- [ ] **Networking Core**
    - [ ] Update `receive_messages` to use `reader.readline()` instead of `read(1024)`.
    - [ ] Update `send_message` to always append `\n` to outgoing JSON.
    - [ ] Remove `input()` calls (Blocking I/O) from `__init__`.

- [ ] **Handshake Update**
    - [ ] Update `initialise` to send the new `{"t": "H", "n": ..., "k": ...}` packet immediately upon connection.
    - [ ] Add logic to handle the **DIRECTORY** (`DIR`) and **USER JOINED** (`J`) packets from the server.
    - [ ] Store these keys in `self.user_public_keys`.

---

## Phase 3: TUI Integration
*Goal: Connect the backend to the frontend.*

- [ ] **Startup Logic**
    - [ ] **Host Mode:** Textual triggers `asyncio.create_task(server_instance.start_server())`, then connects localhost client.
    - [ ] **Join Mode:** Textual triggers `client.connect(host, port)`.

---

## Phase 4: Hybrid Encryption (The Final Layer)
*Goal: Secure the Direct Messages.*

- [ ] **Encryption Helpers**
    - [ ] Implement `encrypt_packet(target_key, content)`: AES gen -> Encrypt Content -> Encrypt Key (RSA) -> Base64.
    - [ ] Implement `decrypt_packet(packet)`: Base64 Decode -> Decrypt Key (RSA) -> Decrypt Content.
- [ ] **Integration**
    - [ ] Apply helpers to the `send` and `receive` pipelines for DM packets.
