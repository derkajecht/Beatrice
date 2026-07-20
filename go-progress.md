# Go Rewrite Progress Tracker

## Overview

| Package | Status |
|---|---|
| `internal/types` | Done |
| `internal/models` | Mostly done |
| `internal/validation` | Done |
| `internal/listener` | Done |
| `internal/utils` | Done |
| `internal/storage` | Schema done, no CRUD |
| `internal/api` | Partially done |
| `internal/crypto` | **Not started** |
| `internal/tui` | **Not started** |
| `cmd/client` | Patched, but broken |
| `cmd/server` | Patched, but broken |
| Tests | Minimal |

---

## Phase 0: Fix Critical Bugs First

- [x] **Client uses `net.Listen` instead of `net.Dial`** (`internal/api/connection.go:55`) — `NewClient` is a copy of `NewServer`. Replace with `net.Dial("tcp", address)`.

- [x] **`NewClient` validation logic inverted** (`internal/api/connection.go:63`) — `if !validation.IsEmpty(addr)` rejects non-empty values. Should be `if validation.IsEmpty(addr)`.

- [x] **`NewServer` argument order swapped** — signature is `(port, host)` but callers pass `(host, port)`. Fix either the signature or the call sites.

- [x] **Server blocks forever before DB init** (`cmd/server/main.go:17`) — `api.NewServer` runs an infinite accept loop. `storage.NewDatabase` on the next line never executes. Need goroutine or restructure.

- [x] **Handshake loop exits after first client** (`internal/utils/router.go:50`) — `return false, nil` is inside the `for` loop over `ChatRoom.Clients`, so only the first client is ever examined. Move it outside the loop.

---

## Phase 1: App Launch & Identity

- [ ] **TUI framework** — Set up `charmbracelet/bubbletea` + `lipgloss` + `bubbles` in `internal/tui/`
- [ ] **Login screen** — Nickname prompt (matching the Python version's login screen with ASCII art)
- [ ] **Key generation on first launch** — RSA-2048 key pair generation in `internal/crypto/`
- [ ] **Key persistence** — Save/load private key from `~/.config/beatrice/private_key.pem` (PKCS#8 PEM)
- [ ] **Returning user detection** — Check if key file exists, skip key gen if so

---

## Phase 2: New User Registration (HTTP API)

- [ ] **HTTP server** — Set up `net/http` or Gin for REST endpoints
- [ ] **`POST /login`** — Accept `{nickname, public_key}`
    - [ ] Validate nickname (alphanumeric, 3-20 chars)
    - [ ] Validate public key is valid PEM
    - [ ] Check `users` table for duplicate nickname
    - [ ] Insert new user or verify existing key matches
    - [ ] Return `201 Created` or error
- [ ] **`POST /login` in `NewClient`** — Send HTTP handshake before WebSocket connect

---

## Phase 3: Message History Sync

- [ ] **`GET /messages/history?nickname=&days=`** — Query messages table for offline messages
- [ ] **Client-side history fetch on reconnect** — Pull and decrypt missed messages
- [ ] **CRUD operations in `internal/storage/db.go`**:
    - [ ] `SaveUser(db, nickname, pubkey) error`
    - [ ] `GetUser(db, nickname) (User, error)`
    - [ ] `SaveMessage(db, sender, recipient, iv, encKey, encContent, sig) error`
    - [ ] `GetMessages(db, nickname, days) ([]Message, error)`
    - [ ] `DeleteOldMessages(db, retentionDays) error`
- [ ] **Fix `IsValidUsername`** — Query uses `username` column but table has `nickname`

---

## Phase 4: WebSocket Handshake & Live Presence

- [ ] **WebSocket upgrade** — Replace raw TCP with `gorilla/websocket` in both client and server
- [ ] **WebSocket endpoint** — `ws://<host>:<port>/ws/<nickname>`
- [ ] **Connection manager** — Track active connections (map of nickname → ws connection), matching Python's `ConnectionManager`
- [ ] **Join broadcast** — Send `JoinPacket` to all other users when someone connects
- [ ] **Leave handling** — Send `LeavePacket`, remove user from `ChatRoom.Clients`, close connection
- [ ] **Abrupt disconnect detection** — Handle closed connections via WebSocket close frame

---

## Phase 5: Sending Messages

- [ ] **`HandleMessage`** (`internal/utils/router.go:78`) — Implement message routing logic
- [ ] **Message input** in TUI — Input widget with `/@Nickname message` DM support
- [ ] **Encryption** (`internal/crypto/`):
    - [ ] AES-256-GCM symmetric encryption
    - [ ] RSA-OAEP (SHA-256) asymmetric key wrapping
    - [ ] RSA-PSS signing (MGF1/SHA-256)
    - [ ] Per-recipient encrypted key (broadcast = N RSA ops)
    - [ ] Base64 encoding for wire format fields
- [ ] **Send flow**: encrypt → build `MessagePacket` → send via WebSocket
- [ ] **Message persistence** — Server inserts into `messages` table on receive
- [ ] **Self-message prevention** — Block `@selfnickname` at client level

---

## Phase 6: Receiving Messages

- [ ] **Decryption** (`internal/crypto/`):
    - [ ] RSA-OAEP decrypt AES key
    - [ ] AES-256-GCM decrypt payload
    - [ ] RSA-PSS verify signature
    - [ ] Replay attack prevention (seen nonces cache with maxlen 2000)
- [ ] **Receive flow**: WebSocket read → decrypt → verify → append to TUI chat log
- [ ] **Payload padding** — Pad to 4096 bytes with random data for traffic analysis mitigation
- [ ] **Key fingerprint display** — SHA-256 of pubkey, first 8 hex chars as `XXXX:XXXX`

---

## Phase 7: Maintenance

- [ ] **Retention sweep** — Periodic goroutine deleting messages older than N days
- [ ] **Inactivity tracking** — Track last interaction time (TUI events), optional away status

---

## TUI Screens (from Python version)

- [ ] **Chat log** — `VerticalScroll` with sent/received/DM messages styled differently (blue/grey borders)
- [ ] **Online users panel** — Docked right panel showing connected users
- [ ] **Message input** — Bottom bar input widget
- [ ] **System messages** — Join/leave/directory notifications (centered, muted color)
- [ ] **Timestamp labels** — `[DD/MM/YY HH:MM]` on first daily message, `[HH:MM]` after
- [ ] **Key bindings** — Ctrl+C to quit, Ctrl+Q disabled
- [ ] **Welcome label** — `"Welcome to Beatrice, {nickname}!"`
- [ ] **Responsive layout** — Hide online panel on narrow terminals (<75 cols)

---

## Wire Protocol Packets (JSON envelope)

All packets wrapped in `{"t": <type>, "m": <inner_json>}`:

| Type | Packet | Direction | Status |
|---|---|---|---|
| `h` | Handshake | Client → Server | Partial (no challenge-response) |
| `m` | Message | Bidirectional | Not started |
| `j` | Join | Server → All | Not started |
| `l` | Leave | Server → All | Not started |
| `d` | Directory | Server → Client | Partial (sent but content type?) |
| `e` | Error | Server → Client | Partial (only 1/6 errors handled) |
| `c` | Challenge | Server → Client | Not started |

---

## Tests

- [ ] `TestNewServer_EmptyPort` — Fix argument order
- [ ] `TestNewServer_ValidInput` — Valid host/port
- [ ] `TestNewClient` — Use `net.Dial` not `net.Listen`
- [ ] `TestHandleHandshake` — Multi-user, dup nickname, empty fields
- [ ] `TestHandleMessage/Join/Leave/Error/Dir/Challenge`
- [ ] `TestGetUserList` — Currently an empty stub
- [ ] Crypto unit tests (encrypt/decrypt round-trip, sign/verify)
- [ ] Database CRUD tests

---

## Infrastructure

- [ ] **CLI flags** — Switch from `flag` to `cobra` (per go-rewrite.md)
- [ ] **Config management** — Add `viper` for config file support
- [ ] **TCP framing** — Fix `listener.go` to handle partial/coalesced TCP reads (delimiter or length-prefix)
- [ ] **Read timeouts** — Add read deadlines to prevent hung goroutines
- [ ] **Max message size** — Enforce a limit on incoming messages
