# Beatrice 🐶
> A terminal-based TUI chat app that has absolutely no idea what you're talking about.

For developers, or anyone who spends most of their time in a terminal, why should you have to leave to talk to your team? Beatrice is a TUI chat app that keeps you in your workflow.

And, thanks to its end-to-end encryption, it has *absolutely no idea* what you're talking about.

---

## 🚧 Project Status: Pre-Alpha

**This project is currently in the very early stages of development.**

It's a core part of my MSc in Computer Science, with a focus on network programming and cybersecurity. The immediate goal is to build a robust, secure backend. The TUI (using `textual` or `curses`) will be built on top of that.

**Current working features (as of Nov 2025):**
* Basic `socket` client and server connection on `localhost`.
* Initial `stack` data structure for command history.

---

## ✨ Planned Features

The end goal is to build a fully-featured, secure chat client. The roadmap includes:

* **🔒 End-to-End Encryption:** Using Python's `ssl` module to implement a standard **TLS handshake**. This will use a combination of asymmetric (like **RSA** or **ECC**) and symmetric (like **AES**) encryption to secure all messages.

* **👥 Contact List:** The ability to add and manage your contacts.
* **🗂️ File Transfers:** Securely send files to your contacts.
* **💻 Multiplexer Integration:** Play nicely with terminal multiplexers like `tmux`.
* **🐙 GitHub Integration:** Potentially for authentication or sharing code snippets.

---

## 🚀 How to Run (Development)

This project is not yet packaged. To run it locally, you must run the server and client in separate terminal windows.

1.  **Clone the repository:**
    ```bash
    git clone [https://github.com/](https://github.com/)[your_username]/beatrice.git
    cd beatrice
    ```
2.  **Run the server:**
    ```bash
    python server.py
    ```
3.  **In a new terminal, run the client:**
    ```bash
    python client.py
    ```

---

## 🐶 The Story: What's an "Encryption Dog"?

Beatrice is the name of my wife and I's Jack Russell puppy. She's wonderful.

For a puppy, she's very quiet, and when I'm singing to her, or ranting about my day at work, she just looks up at me and then immediately gets back to chewing my hoodie or the desk or my shoes...

I wondered if she was listening, or just waiting until her next meal, or walkies. She's probably thinking, "what on earth are you talking about?"

And that's the bit that got me. **Encryption dog.**

I speak to her, but she's got no idea what I'm saying. This chat app is the same. It happens entirely within your terminal, and *it has absolutely no idea what you are talking about.*
