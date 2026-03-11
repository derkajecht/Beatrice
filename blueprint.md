## Phase 1: Application Launch & Identity Resolution
*Action happens strictly on the Client (Local Machine).*

1. **Initialize TUI:** The user launches the Textual application.
2. **Key Check:** The client searches its local file system (e.g., `~/.beatrice/private.pem`) for an existing Private Key.
    * **Branch A (Key Found):** The system recognizes a **Returning User**. Proceed to **Phase 3**.
    * **Branch B (No Key):** The system recognizes a **New User**. Proceed to **Phase 2**.

---

## Phase 2: New User Onboarding
*Action involves both Client and Server. (Only executes if Branch 1B was taken).*

1. **Nickname Prompt:** The Textual UI prompts the user to choose a nickname.
2. **Key Generation (Client):** The client generates a new Asymmetric Key Pair (Private Key + Public Key) in memory.
3. **Local Save (Client):** The client saves the Private Key securely to the local file system.
4. **Registration Payload (Client):** The client formats an HTTP POST request containing the chosen `nickname` and the `public_key` (in PEM format).
5. **Registration Handling (Server):** * The server receives the POST request.
    * The server checks the **Users Table** (SQLite) to ensure the nickname is not taken.
    * The server inserts a new row: `[nickname, public_key]`.
    * The server returns a `201 Created` HTTP success response.
6. **Skip History:** Because the user is new, there is no history to fetch. **Skip Phase 3 and proceed to Phase 4.**

---

## Phase 3: Returning User History Sync
*Action involves both Client and Server. (Only executes if Branch 1A was taken).*

1. **Nickname Prompt:** The Textual UI prompts the user for their nickname. (Alternatively, the client can read the nickname from a local config file saved alongside the Private Key).
2. **Sync Request (Client):** The client sends an HTTP GET request to the server's `/messages/history` endpoint, providing their `nickname` and a `days_to_keep` parameter (e.g., 30).
3. **Database Query (Server):** * The server calculates the cutoff date (`Current Time - 30 Days`).
    * The server queries the **Messages Table** for all rows where `recipient_nickname` matches the user AND `timestamp` is newer than the cutoff.
    * The server returns an array of these encrypted message objects as JSON.
4. **Decryption (Client):** * The client iterates over the JSON array.
    * For each message, it uses the local Private Key to decrypt the `ciphertext`.
5. **UI Population (Client):** The client pushes the decrypted plaintext messages into the Textual UI log. Proceed to **Phase 4**.

---

## Phase 4: WebSocket Handshake & Live Presence
*Action bridges Client and Server.*

1. **Connection Request (Client):** The client initiates a WebSocket connection to the server's endpoint (e.g., `ws://<server_ip>/ws/<nickname>`).
2. **Connection Acceptance (Server):** * The server accepts the upgrade request.
    * The server adds the user to its **Active Connections Map** in RAM (e.g., `active_connections["Alice"] = websocket_object`).
3. **Ready State:** The application is now fully initialized. The user can view the UI and type new messages.

---

## Phase 5: Sending a Live Message
*Triggered when the user hits "Enter" in the chat input.*

1. **Recipient Key Lookup (Client):** The user types a message intended for a specific recipient (e.g., "Bob"). The client checks its local RAM cache for Bob's Public Key.
    * **Sub-branch (Cache Miss):** If Bob's key is not cached, the client pauses, sends an HTTP GET `/users/Bob/key` to the server, receives the Public Key, caches it, and resumes.
2. **Encryption (Client):** The client encrypts the plaintext message using Bob's Public Key.
3. **Transmission (Client):** The client sends a JSON payload through the open WebSocket: `{"recipient": "Bob", "ciphertext": "<encrypted_string>"}`.
4. **Persistence (Server):** * The server receives the WebSocket frame.
    * The server stamps it with the current database time.
    * The server inserts a new row into the **Messages Table**: `[sender, recipient, ciphertext, timestamp]`.
5. **Routing (Server):** The server checks the **Active Connections Map** for the recipient ("Bob").
    * **Sub-branch (Recipient Online):** If Bob is in the map, the server immediately pushes the JSON payload down Bob's WebSocket.
    * **Sub-branch (Recipient Offline):** If Bob is not in the map, the server does nothing. (Bob will pull this message from the database during Phase 3 next time he logs in).

---

## Phase 6: Receiving a Live Message
*Triggered when a payload arrives through the client's WebSocket.*

1. **Frame Reception (Client):** The client receives the JSON payload from the server.
2. **Decryption (Client):** The client uses its local Private Key to decrypt the `ciphertext`.
3. **UI Update (Client):** The client appends the newly decrypted plaintext message to the Textual UI log.

---

## Phase 7: Automated Server Maintenance
*Triggered periodically by the server (e.g., daily background task).*

1. **Retention Sweep (Server):** The server runs a `DELETE` SQL query on the **Messages Table**.
2. **Purge (Server):** All rows where the `timestamp` is older than the defined retention threshold (e.g., 30 days) are permanently removed from the SQLite file to save space and protect privacy.





Updated Plan: Beatrice Restructuring with Console Commands
Current Issues

- beatrice.py (root) shadows the beatrice/ package
- beatrice/client.py uses absolute import for crypto_utils
- No console script entry point in pyproject.toml
- server.py doesn't have a callable main() function
Steps
1. Rename Entry Point File
Action: Rename /home/jordan/Documents/Beatrice/beatrice.py → /home/jordan/Documents/Beatrice/main.py
Why: Avoids package naming conflict with beatrice/ directory
2. Fix Import in Entry Point
In main.py (line 12), change:

FROM:

from beatrice.client import Client

TO:

from src.client import Client
3. Fix Relative Imports in Package
In src/client.py (line 21), change:

FROM:

from crypto_utils import check_or_create_keys, get_public_key_bytes

TO:

from .crypto_utils import check_or_create_keys, get_public_key_bytes
In src/server.py, fix any similar absolute imports to use relative imports (.module)

4. Add Main Function to Entry Point
   In main.py, wrap the bottom code:
   def main():
    app = Beatrice()
    app.run()
   if __name__ == "__main__":
    main()
5. Add Main Function to Server
   In src/server.py, wrap the bottom code in a main() function:
   def main():
    # Move all the argument parsing and server startup code here
    parser = argparse.ArgumentParser(...)
    # ... rest of the server setup code ...
    uvicorn.run(app, host=args.host, port=args.port)
   if __name__ == "__main__":
    main()
6. Create __main__.py
Create file: /home/jordan/Documents/Beatrice/src/__main__.py
from .main import main
main()
This enables: python -m src
7. Add Console Script Entry Points
In pyproject.toml, add:
[project.scripts]
beatrice = "src.main:main"
beatrice-server = "src.server:main"
8. Optional: Rename src/ to beatrice/
If you prefer the package named beatrice instead of src:
- Rename directory src/ → beatrice/
- Update imports: from src.client → from beatrice.client
- Update pyproject.toml:
[project.scripts]
beatrice = "beatrice.main:main"
beatrice-server = "beatrice.server:main"
9. Install Package
Run:
uv pip install -e .
After this, users can run:
- beatrice (starts the client)
- beatrice-server (starts the server)

---
