# Project Roadmap: Secure Chat Application

**Current Status:** ~85% Functional / ~60% Portfolio-Ready
**Goal:** Reach 100% "Hiring Standard" for a Security Engineer Portfolio.

---

## Phase 1: Security Hardening (Critical)
*These features demonstrate your understanding of cryptographic integrity and threat modeling.*

- [ ] **1. Implement Replay Protection**
    - [ ] Update `client.py` (sender): Add `timestamp` (UTC) and `nonce` (random hex) to the message payload dictionary.
    - [ ] Update `client.py` (receiver): Reject messages where `timestamp` is older than 60 seconds.
    - [ ] Update `client.py` (receiver): Maintain a set of `seen_nonces` and reject duplicates.

- [ ] **2. Implement Key Fingerprinting (MITM Protection)**
    - [ ] Update `client.py`: Create a helper method `get_fingerprint(public_key)` that returns the first 8 characters of the SHA-256 hash (e.g., `a1b2:c3d4`). DONE
    - [ ] Update `app.py`: Modify the "Online Users" sidebar to display the fingerprint next to the nickname (e.g., `Alice [a1b2]`).
    - [ ] *Why:* Allows users to visually verify they are talking to the correct person, mitigating Man-in-the-Middle attacks.

- [ ] **3. Traffic Padding (Metadata Protection)**
    - [ ] Update `client.py` (sender): Pad the JSON payload string with whitespace so its total byte length is a multiple of 256 bytes.
    - [ ] Update `client.py` (receiver): Strip whitespace from the decrypted JSON before parsing.
    - [ ] *Why:* Prevents traffic analysis where an attacker guesses the message content based on packet length.

---

## Phase 2: Resilience & Stability
*These features make the application robust against real-world network issues.*

- [ ] **4. Handle "Zombie" Connections**
    - [ ] Update `server.py`: Wrap `writer.write()` in a `try/except` block.
    - [ ] Logic: If a `BrokenPipeError` or `ConnectionResetError` occurs, immediately remove the user from `connected_users` and close the socket.

- [ ] **5. Graceful Shutdown**
    - [ ] Update `app.py`: Catch `Ctrl+C` or the application exit event.
    - [ ] Logic: Send a final "Leave Packet" to the server so other users are immediately notified that you have gone offline.

- [ ] **6. Input Sanitization**
    - [ ] Update `app.py`: Ensure the nickname validator rejects special characters (like `/`, `\`, `:`, etc.) that could break file paths or UI formatting.

---

## Phase 3: The "Hiring Manager" Polish
*Documentation and code quality are what separate students from engineers.*

- [ ] **7. Create `SECURITY.md`**
    - [ ] Document the **Threat Model**: Explicitly state that the server is assumed to be "Honest-but-Curious."
    - [ ] Document **Limitations**: Acknowledge that you are using TOFU (Trust On First Use) and explain how a verified Key Server would improve security in v2.

- [ ] **8. Create `README.md`**
    - [ ] Add a setup guide (How to run Server vs Client).
    - [ ] Add an architecture diagram (Mermaid or ASCII) showing the Hybrid Encryption flow (`AES` for data, `RSA` for keys).

- [ ] **9. Code Quality Sweep**
    - [ ] Add Type Hints to all functions (e.g., `def connect(self) -> bool:`).
    - [ ] Add Docstrings to complex classes explaining *why* you chose specific algorithms (e.g., "Using AES-GCM for authenticated encryption").

---

## Suggested Workflow

1.  **Start with Task #2 (Fingerprinting).** It is low effort but adds a cool visual element to the UI immediately.
2.  **Move to Task #1 (Replay Protection).** This is the core "security engineering" task.
3.  **Finish with Phase 3 (Documentation).** Do not skip this; it is the most important part for your portfolio.
