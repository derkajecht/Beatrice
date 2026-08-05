# Beatrice — Mini Portfolio

> **A privacy-first, end-to-end encrypted chat application that lives entirely in the terminal.**
> The server routes your messages but can *never* read them.

**Stack:** Go · HPKE / post-quantum ML-KEM · WebSockets · SQLite · Bubble Tea (TUI)

---

## Project Overview

Beatrice is a real-time chat app for people who never leave the terminal. Conversation
happens in a TUI, and — thanks to a from-scratch end-to-end encryption layer — the server
routing every message is cryptographically unable to decrypt a single one of them.

It began as a Python/Textual prototype built to learn **network security, async I/O, and
cryptography**, and is now being rewritten in Go with a focus on performance, type safety,
and modern (post-quantum) primitives.

### What makes it interesting

| Area | Why it matters |
| --- | --- |
| **Post-quantum crypto** | Uses **ML-KEM-768 + X25519**, HKDF-SHA512, AES-256-GCM. Ready for the quantum era, today. |
| **Zero-trust server** | The server routes ciphertext it cannot decrypt (@see "noise" security model). |
| **Custom wire protocol** | A typed, versioned envelope for every packet travelling between client and server. |
| **Resilient async client** | Exponential backoff with jitter keeps the client alive through network hiccups. |
| **Concurrent hub** | A single event-loop goroutine handles all client lifecycle without locks. |
| **Encrypted at rest** | Full ciphertext + keys + IVs persisted to SQLite with proper FK constraints. |

---

## Screenshot 1 — The app in action

> **📸 Insert:** a screenshot showing the running TUI — the chat view with the login page
> (or a live two-window demo). Existing shots live in `assets/main_page.png` and
> `assets/chat_page.png`.

This makes the "terminal-native" claim immediately visible to a reviewer before any code.

---

## Screenshot 2 — The post-quantum session construction

**Source:** `internal/client/crypto_utils.go`

```go
func NewCryptoSuite() SuiteConfig {
	return SuiteConfig{
		KEM:  hpke.MLKEM768X25519(),
		KDF:  hpke.HKDFSHA512(),
		AEAD: hpke.AES256GCM(),
		Info: []byte("Beatrice"),
	}
}
```

The heart of the project. Each session spins up an **ephemeral key pair**, derives its
public half, and wires a configurable HPKE suite whose defaults target **post-quantum
security** (ML-KEM-768 hybrid) — with the key exchange itself providing forward secrecy.
The `CryptoPacket` also carries a rolling `SeenNonces` deque and a `KeyCache`, so every
session is one-time and no nonce is ever reused — a classic footgun avoided by design.

> **📸 Insert:** screenshot of `NewUserSession()` + `NewCryptoSuite()` together, highlighting
> the KEM/KDF/AEAD selection and the nonce-replay protection. This is the strongest "I
> understand crypto engineering, not just line-of-business code" signal.

---

## Screenshot 3 — The typed wire protocol

**Source:** `internal/shared/types.go`

```go
type GeneralPacket struct {          // global envelope
	Type    string          `json:"t"`
	Message json.RawMessage `json:"m"`
}

type MessagePacket struct {
	Recipient string `json:"r"`
	Sender    string `json:"s"`
	IV        string `json:"iv"`  // Base64 IV for AES-GCM
	AESKey    string `json:"k"`   // Ephemeral AES key wrapped by recipient's pub key
	Content   string `json:"c"`   // Encrypted payload
	Signature string `json:"sig"` // Non-repudiation check
}
```

Every message is a thin, typed envelope whose inner payload is opaque JSON. The design
keeps the protocol explicit and forward-compatible: the sender's signature proves
authenticity, the per-message AES key is wrapped for the recipient only, and the payload
travels over the socket without the server ever holding a plaintext copy.

> **📸 Insert:** the full `MessagePacket` + `GeneralPacket` structs. Labels walking
> through which bytes are encrypted vs. wrapped vs. public is ideal — it tells the
> story of the encryption model at a glance.

---

## Screenshot 4 — Resilience: exponential backoff with jitter

**Source:** `internal/client/websocket.go`

```go
for {
	conn, _, err := websocket.Dial(ctx, addr, nil)
	if err != nil {
		jitter := time.Duration(rand.Int63n(int64(delay / 2)))
		wait := delay + jitter
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return User{}, ctx.Err()
		}
		delay = min(delay*2, maxDelay)
		continue
	}
	// ...
	client := User{Conn: conn, TuiChan: make(chan []byte, 100)}
	if err := client.readLoop(ctx); err != nil { /* ... */ }
	return client, nil
}
```

Reconnection logic that not only backs off exponentially (up to a cap) but **jitters each
retry** so a burst of reconnecting clients naturally de-synchronises instead of thundering
herding the server. Context-aware throughout, so cancellation aborts a wait cleanly. The
`readLoop` pushes frames into a buffered channel bound for the TUI — a clean, decoupled
producer/consumer shape.

> **📸 Insert:** the `NewChatClient` reconnect loop. Cue attention to the jitter, the
> `min(delay*2, maxDelay)` cap, and the `ctx.Done()` fast path.

---

## Screenshot 5 — Lock-free concurrency on the server hub

**Source:** `internal/server/websocket.go`

```go
case client, ok := <-h.deleteClient:
	if !ok {
		return
	}
	delete(h.clients, client)
	client.CloseNow()
```

The server funnels every lifecycle event — register, broadcast, deregister — through
**Go channels into one listener goroutine**. This is the idiomatic Go way to share state
by communicating rather than locking; no mutexes, no race conditions, and each case is a
linearised decision point. The consumer loop also carries a 30-second read timeout per
wait, so stale connections are cleaned up automatically.

> **📸 Insert:** the `Hub.Listener` central event loop + `wsHandler`'s register/deregister
> wiring. Highlight the channel-based state management as the proof of understanding
> concurrency under fire.

---

## Screenshot 6 — Encrypted-at-rest schema and sanitisation

**Source:** `internal/server/db.go` + `validation.go`

```sql
CREATE TABLE IF NOT EXISTS messages (
	id                 INTEGER PRIMARY KEY AUTOINCREMENT,
	sender             TEXT NOT NULL,
	recipient          TEXT NOT NULL,
	iv                 TEXT NOT NULL,
	encrypted_key      TEXT NOT NULL,
	encrypted_content  TEXT NOT NULL,
	signature          TEXT NOT NULL,
	timestamp          TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (sender) REFERENCES users(nickname)
);
```

The schema stores **nothing but ciphertext and its keying material** — persistence is
encrypted-at-rest by construction. Foreign keys are enabled at the DSN layer, and a
`UsernameSanitizer` strips non-alphanumerics before anything touches the database or wire,
giving defence-in-depth across the whole attack surface.

> **📸 Insert:** the schema `CREATE TABLE` block. Underline that the column list is a
> *threat model on paper* — IVs and wrapped keys are stored, but no plaintext ever is.

---

## Tech Stack

- **Go 1.26** – main rewrite language, static binary server + client
- **crypto/hpke** – ML-KEM-768 with X25519 · HKDF-SHA512 · AES-256-GCM
- **coder/websocket** – long-lived duplex transport
- **modernc.org/sqlite** – pure-Go, embedded persistence (no CGO)
- **charmbracelet/bubbletea + lipgloss** – the TUI
- **[Python origins]** – original asyncio + Textual + pycryptodome prototype, still in `src/`

---

## What I'd love to talk about

1. Choosing **post-quantum primitives while the ecosystem is still standardising** — and
   the trade-offs I made doing so.
2. How the **zero-trust server** shapes the protocol, storage, and UX decisions.
3. The **Go channel-based concurrency model** and how it replaced a lock-heavy Python
   approach — and what the two teach about each other.

*Want to see it run? `go run ./cmd/...` and open two client windows.*