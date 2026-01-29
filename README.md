# Beatrice 🐶
> A terminal-based TUI chat app that has absolutely no idea what you're talking about.

For developers, sysadmins, or anyone who lives in the terminal: why leave your workflow just to talk to your team? Beatrice keeps your communication right where you work.

And, thanks to its custom end-to-end encryption implementation, Beatrice has *absolutely no idea* what you're talking about.

---

## 🚧 Project Status: Educational Prototype

**This project is a functional prototype built for my MSc in Computer Science.**

Its primary goal is to demonstrate core concepts in **Network Security**, **Asynchronous I/O**, and **Cryptography**. Unlike standard chat apps that rely on pre-wrapped TLS libraries, Beatrice implements a custom Hybrid Encryption protocol at the application layer to demonstrate how secure channels are constructed from primitives.

**Current Features:**
* **🖥️ Modern TUI:** Built with [Textual](https://textual.textualize.io/) for a smooth, mouse-compatible terminal experience.
* **⚡ Asynchronous:** Fully non-blocking architecture using Python's `asyncio`.
* **🔒 Hybrid Encryption:** Secures messages using **RSA-2048** for key exchange and **AES-GCM** for message confidentiality and integrity.
* **💬 Real-time Messaging:** Instant messaging with live online user tracking.

---

## 🛡️ Security Model & Limitations

> **Note to Recruiters/Engineers:** This application is an educational implementation of cryptographic protocols.

### The Architecture
1.  **RSA Handshake:** Upon connection, clients exchange public keys with the server.
2.  **AES Session:** Messages are encrypted using AES-GCM (Galois/Counter Mode), ensuring both privacy and integrity.
3.  **Honest-but-Curious Server:** The server routes encrypted bytes but cannot decrypt the content (assuming no active MITM).

### 📍 Project Roadmap
This project is under active development. The following features are next on the agenda to move from "Prototype" to "Production-Ready":

* [ ] **Data Persistence:** Currently, chat history is ephemeral and clears when the server restarts. A database (SQLite/PostgreSQL) implementation is planned to store encrypted message blobs.
* [ ] **Replay Protection:** Implementing timestamped nonces to prevent packet replay attacks.
* [ ] **Identity Verification:** Adding visual key fingerprints to mitigate Man-in-the-Middle (MITM) attacks (currently relies on Trust-On-First-Use).
* [ ] **Traffic Padding:** Padding payloads to fixed lengths to prevent metadata/traffic analysis.

---

## 🚀 How to Run

This project requires **Python 3.10+**.

### 1. Installation
Clone the repository and install the dependencies (Textual and Cryptography).

```bash
git clone [https://github.com/derkajecht/beatrice.git](https://github.com/your_username/beatrice.git)
cd beatrice
pip install textual cryptography
```

### To start the server - do this first!
```bash
python server.py
```

### To start the main app, open a new terminal window and run the following;
```bash
python beatrice.py
```
*Enter a nickname when prompted. You can open multiple beatrice.py client windows to test messaging between them :)*

## 🐶 The Story: What's an "Encryption Dog"?

Beatrice is the name of my wife and I's Jack Russell puppy. She's wonderful.

For a puppy, she's very quiet. When I'm singing to her, or ranting about my day at work, she just looks up at me and then immediately gets back to chewing my hoodie, the desk, or my shoes...

I often wonder if she's listening, or just waiting until her next meal. She's probably thinking, "What on earth are you talking about?"

And that's the bit that got me. Encryption dog.

I speak to her, but she's got no idea what I'm saying. This chat app is the same. It happens entirely within your terminal, and it has absolutely no idea what you are talking about.
