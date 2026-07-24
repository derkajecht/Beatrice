# Go Rewrite — Code Review & Action Plan

> **Context:** The Python implementation will be deprecated. The Go rewrite is a standalone app
> with its own wire protocol, crypto, and architecture. No backward compatibility required.

---

## Table of Contents

1. [Critical Bugs (P0 — blocking execution)](#1-critical-bugs)
2. [Design Issues (P2 — structural)](#2-design-issues)
3. [Redundant / Dead Code](#3-redundant--dead-code)
4. [Test Issues](#4-test-issues)
5. [Architecture & Wire Protocol](#5-architecture--wire-protocol)
6. [Execution Plan](#6-execution-plan)

---

## 1. Critical Bugs

### 1.1 `sync.Pool` type mismatch causes panic

**File:** `internal/listener/listener.go:34,142`

Pool's `New` factory creates `[]byte` (a slice header), but `Put(&buf)` passes `*[]byte` (pointer). After one cycle, `Get().([]byte)` receives a `*[]byte` and the type assertion panics.

**Fix:** Change `bufferPool.Put(&buf)` to `bufferPool.Put(buf)`.

---

### 1.2 Deadlock: `HandleHandshake` + `GetUserList` both lock `ChatRoom`

**Files:** `internal/utils/router.go:20`, `internal/models/packets.go:34`

`HandleHandshake` acquires `types.ChatRoom.RLock()` then calls `models.GetUserList()`, which calls `r.Lock()`. Go's `sync.RWMutex` does not allow read→write upgrade — this deadlocks on the second concurrent connection.

**Fix:** Drop the lock in `HandleHandshake` before calling `GetUserList`, or add a non-locking `GetUserListLocked` variant.

---

### 1.3 Server exits immediately at startup

**File:** `cmd/server/main.go:26`

`go api.NewServer(host, port, conType)` launches in a goroutine, but `RunServer` returns immediately. `main()` falls through and exits before the TCP listener accepts anything.

**Fix:** Add `select{}` at the end of `main()` or use `sync.WaitGroup`.

---

### 1.4 `NewClient` infinite reconnect loop

**File:** `internal/api/connection.go:81-90`

The `for {}` loop dials, handles (returns when connection closes), then immediately dials again with no backoff, stop condition, or cancellation. Spams the server on transient failure.

**Fix:** Remove `for {}` — establish a single connection. Add exponential backoff if reconnect is desired.

---

### 1.5 `IsValidUsername` queries wrong column

**File:** `internal/validation/validation.go:63`

Query: `SELECT id FROM users WHERE username = ?`

Schema (`internal/storage/db.go:76`): column is `nickname`, not `username`.

**Fix:** Change `username` to `nickname` in the query.

---

### 1.6 Inconsistent error packet wire format

**Files:** `internal/utils/responses.go:15-36` vs `internal/utils/responses.go:40-77`

`SendError` writes a bare `{"m":"..."}` to the wire. `SendPacketToClient` wraps in an envelope: `{"t":"e","m":{"m":"..."}}`. The same logical error takes two different wire shapes depending on which function sent it.

**Fix:** Route all errors through `SendPacketToClient` with an envelope. Remove `SendError` or make it a thin wrapper.

---

## 2. Design Issues

### 2.1 Buffer pool reuse race

**File:** `internal/listener/listener.go:16-20,140-142`

buf is zeroed with `clear(buf)` after `Put` into the pool. Another goroutine could `Get` the buffer while it's being zeroed. Also the byte-by-byte zeroing in the commented-out `ClearBuffer` was unnecessary — `clear()` is sufficient.

**Fix:** Put buffers back clean, or just rely on `clear()` before Put which already exists. After fixing the `&buf` bug, the flow is: pool creates `[]byte`, listener reads into it, clears it, puts it back.

### 2.2 Connection dedup by net.Conn pointer

**File:** `internal/utils/router.go:24`

`GetClient(conn)` looks up by `net.Conn` interface value. A reconnecting client gets a new connection — the "already connected" check can never fire on reconnect.

**Fix:** Check by nickname instead of `net.Conn` pointer. Add a `GetClientByNickname` method.

---

### 2.3 `NewDatabase` returns unused path

**File:** `internal/storage/db.go:38`

Returns `(*sql.DB, string, error)` — the third return value (file path) is assigned but never read by any caller.

**Fix:** Remove the third return value, or make the caller construct the path and pass it in.

---

### 2.4 No read/write deadlines on connections

**File:** `internal/listener/listener.go:37`

`conn.Read(buf)` blocks indefinitely — a hung client pins a goroutine forever.

**Fix:** Add `conn.SetReadDeadline(time.Now().Add(readTimeout))` before each read.

---

### 2.5 No payload size limit

**File:** `internal/listener/listener.go:38`

The 4096-byte buffer silently truncates larger messages. Messages smaller than 4096 contain stale data from the pool reuse.

**Fix:** Length-prefix messages, or switch to WebSocket (which handles framing natively).

---

### 2.6 `Pingdb()` ignores configured database path

**File:** `internal/storage/db.go:17-33`

`Pingdb()` always calls `config.NewDatabaseInfo()` which returns the hardcoded default `"./beatrice"`. It ignores the path configured in `NewDatabase()`. Any caller of `Pingdb()` opens the wrong database.

**Fix:** Accept the DSN/path as a parameter instead of hardcoding.

---

### 2.7 `NewUserSession()` return values discarded

**File:** `cmd/client/main.go:16`

```go
_, _, _, err := crypto.NewUserSession()
```

The generated keys and crypto state are discarded. Only the error is checked.

**Fix:** Store the return values and pass them to the TUI/connection layer.

---

### 2.8 `ServerStruct()` return value discarded

**File:** `cmd/server/main.go:42**

```go
ServerStruct(dbLocation)
```

Creates a `*types.Server` but discards it. The `types.Server` struct's fields (`ActiveConnections`, `PendingConnections`, `DatabasePath`) are never read or written anywhere in the codebase.

**Fix:** Remove the type or wire it into the server state.

---

## 3. Redundant / Dead Code

### 3.1 Entire packages that are dead

| Package | File | Reason |
|---------|------|--------|
| `internal/errors/` | `errors.go` | `SendErrorPacket` never called. Duplicates `utils.SendPacketToClient`. Entire package unreferenced. |
| `internal/auth/` | `auth.go` | `ValidateNickname` and `ValidatePubKey` both hardcoded `return true`. Never imported anywhere. |

### 3.2 Duplicated function

| Function | Location A | Location B | Verdict |
|----------|-----------|-----------|---------|
| `UnmarshalPacket` | `internal/utils/helpers.go:19-30` | `internal/validation/validation.go:88-99` | Byte-for-byte identical. `validation` copy has **zero callers**. Delete it. |

### 3.3 Unused functions (defined, never called)

| Function | File | Line |
|----------|------|------|
| `models.NewClientPacket` | `internal/models/packets.go` | 8 |
| `storage.StoreUser` | `internal/storage/db.go` | 107 |
| `validation.IsEmpty` | `internal/validation/validation.go` | 23 |
| `types.ConvertPacketToMap` | `internal/types/conversion.go` | 7 |
| `logger.LoggerSetup` | `internal/logger/logger.go` | 24 |

### 3.4 Unused types

| Type | File | Line | Notes |
|------|------|------|-------|
| `types.SuccessPacket` | `internal/types/types.go` | 58 | Never instantiated anywhere |
| `logger.ChannelWriter` | `internal/logger/logger.go` | 6 | Struct + Write method unreachable |

### 3.5 Stub functions (TODO-only or hardcoded returns)

**Hardcoded stubs (always return same value, no real logic):**

- `auth.ValidateNickname` — `return true`
- `auth.ValidatePubKey` — `return true`

**Empty TODO shells:**

- `utils.HandleMessage` — `// TODO: Implement message logic`
- `utils.HandleJoin` — `// TODO: Implement join logic`
- `utils.HandleLeave` — `// TODO: Implement leave logic`
- `utils.HandleDir` — `// TODO: Implement dir logic`
- `utils.HandleChallenge` — `// TODO: Implement challenge logic`
- `utils.HandleError` — switch with 6 TODO cases
- `tests.TestGetUserList` — `// TODO: Implement test for GetUserList`

### 3.6 Dead code blocks

| What | Where | Line |
|------|-------|------|
| Commented-out `ClearBuffer` function | `internal/utils/helpers.go` | 10-15 |
| Orphaned `func main()` in `package logger` | `internal/logger/logger.go` | 32-44 |
| Discarded `ServerStruct` return value | `cmd/server/main.go` | 42 |
| Discarded crypto return values | `cmd/client/main.go` | 16 |

### 3.7 Test package conflict

**Files in `tests/` declare different package names:**

- `connections_test.go` → `package tests_test`
- `user_test.go` → `package tests`

Go requires all `.go` files in the same directory to belong to the same package. This won't compile.

---

## 4. Test Issues

| Issue | File | Detail |
|-------|------|--------|
| Wrong package name | `tests/connections_test.go` | `package tests_test` vs `package tests` |
| Inverted args | `tests/connections_test.go:10` | `NewServer("", "localhost", "tcp")` — empty host, not empty port |
| Empty stub | `tests/user_test.go` | `TestGetUserList` has only a TODO comment |

---

## 5. Architecture & Wire Protocol

### Key decisions (Python deprecated, no compat needed)

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Transport | TBD — raw TCP vs WebSocket | Raw TCP simpler deps; WS gives framing + ping/pong for free |
| Crypto | HPKE (MLKEM768X25519 + HKDF-SHA512 + AES-256-GCM) | Already written, post-quantum, modern |
| Auth | Challenge-response over the single connection | No HTTP REST layer needed |
| History sync | Over the main connection on reconnect | No HTTP GET /messages/history needed |
| Packet format | Flat envelope `{"t":"...", ...}` or nested `{"t":"...", "m":{...}}` | Unify to one format |

### Current protocol state

| Packet Type | Direction | Status |
|-------------|-----------|--------|
| Handshake (`h`) | Client → Server | Partially implemented |
| Message (`m`) | Bidirectional | **Stub** — routing not written |
| Join (`j`) | Server → All | **Stub** |
| Leave (`l`) | Server → All | **Stub** |
| Directory (`d`) | Server → Client | Sent (in handshake) |
| Error (`e`) | Server → Client | Works but has wire format inconsistency |
| Challenge (`c`) | Server → Client | **Stub** |

### Recommended architecture

```
                          ┌──────────────┐
                          │   cmd/server  │
                          │   main.go     │
                          └──────┬───────┘
                                 │
                    ┌────────────┴────────────┐
                    │      internal/api        │
                    │  NewServer / NewClient   │
                    └────────────┬────────────┘
                                 │
              ┌──────────────────┼──────────────────┐
              │                  │                    │
     ┌────────┴────────┐ ┌──────┴───────┐ ┌─────────┴─────────┐
     │ internal/listener│ │internal/utils │ │ internal/storage   │
     │ HandleConnection │ │  router.go   │ │ db.go (SQLite)     │
     │  (WS or TCP)    │ │  responses   │ │ CRUD ops           │
     └─────────────────┘ └──────┬───────┘ └───────────────────┘
                                │
              ┌─────────────────┼─────────────────┐
              │                 │                   │
     ┌────────┴────────┐ ┌─────┴──────┐  ┌─────────┴─────────┐
     │  internal/types  │ │internal/crypto│ │  internal/tui      │
     │  packet structs  │ │ HPKE ops   │  │  Bubble Tea app    │
     └─────────────────┘ └────────────┘  └───────────────────┘
```

---

## 6. Execution Plan

### Phase 0 — Fix P0 Bugs (~30 min)

| # | Task | Files | Depends on |
|---|------|-------|------------|
| 0.1 | Fix `sync.Pool` Put type | `listener.go` | — |
| 0.2 | Fix deadlock in HandleHandshake | `router.go`, `packets.go` | — |
| 0.3 | Add `select{}` to server main | `cmd/server/main.go` | — |
| 0.4 | Remove infinite reconnect loop | `api/connection.go` | — |
| 0.5 | Fix DB column name | `validation/validation.go` | — |
| 0.6 | Unify error wire format | `utils/responses.go` | — |
| 0.7 | Fix test package conflict | `tests/*` | — |
| 0.8 | Fix inverted test args | `tests/connections_test.go` | — |

### Phase 1 — Clean up dead code (~15 min)

| # | Task | Files | Depends on |
|---|------|-------|------------|
| 1.1 | Delete `internal/errors/` | `errors/errors.go` | 0.6 |
| 1.2 | Delete `internal/auth/` | `auth/auth.go` | — |
| 1.3 | Delete duplicate `UnmarshalPacket` | `validation/validation.go` | — |
| 1.4 | Delete unused types/functions | `types/conversion.go`, `models/packets.go`, etc. | — |
| 1.5 | Fix `Pingdb()` to accept path param | `storage/db.go` | — |
| 1.6 | Remove unused imports after cleanup | various | 1.1-1.5 |

### Phase 2 — Wire protocol decision + rewrite (~1 session)

**Decision point:** Raw TCP vs WebSocket.

| # | Task | Files | Depends on |
|---|------|-------|------------|
| 2.1 | Choose transport (recommend WebSocket) | — | — |
| 2.2 | If WebSocket: swap listener + api | `listener.go`, `api/connection.go` | 2.1 |
| 2.3 | Add ConnectionManager (map[nickname]conn) | `internal/connection/` or `types/` | 2.2 |
| 2.4 | Add read deadlines / ping-pong | `listener.go` | 2.2 |

### Phase 3 — Implement router handlers

| # | Task | Files | Depends on |
|---|------|-------|------------|
| 3.1 | Implement HandleMessage | `router.go` | 2.3 |
| 3.2 | Implement HandleJoin (broadcast) | `router.go` | 2.3 |
| 3.3 | Implement HandleLeave (broadcast) | `router.go` | 2.3 |
| 3.4 | Implement HandleDir | `router.go` | 2.3 |
| 3.5 | Implement HandleChallenge (auth) | `router.go` | 2.3 |
| 3.6 | Implement HandleError cases | `router.go` | 2.3 |

### Phase 4 — Database CRUD

| # | Task | Files | Depends on |
|---|------|-------|------------|
| 4.1 | `SaveUser(db, nickname, pubkey)` | `storage/db.go` | 1.5 |
| 4.2 | `GetUser(db, nickname)` | `storage/db.go` | 1.5 |
| 4.3 | `SaveMessage(db, sender, recipient, ...)` | `storage/db.go` | 1.5 |
| 4.4 | `GetMessages(db, nickname, days)` | `storage/db.go` | 1.5 |
| 4.5 | `DeleteOldMessages(db, retentionDays)` | `storage/db.go` | 1.5 |

### Phase 5 — Crypto completion

| # | Task | Files | Depends on |
|---|------|-------|------------|
| 5.1 | Key persistence (save/load PEM) | `internal/crypto/` | — |
| 5.2 | HPKE encrypt/decrypt round-trip | `internal/crypto/` | — |
| 5.3 | Sign/verify with HPKE | `internal/crypto/` | — |
| 5.4 | Nonce replay cache | `internal/crypto/` | — |
| 5.5 | Payload padding (traffic analysis) | `internal/crypto/` | — |

### Phase 6 — TUI (Bubble Tea)

| # | Task | Files | Depends on |
|---|------|-------|------------|
| 6.1 | Add Bubble Tea + Lipgloss deps | `go.mod` | — |
| 6.2 | Login screen (nickname prompt) | `internal/tui/` | — |
| 6.3 | Chat log view (scrollable) | `internal/tui/` | 6.2 |
| 6.4 | Online users panel (right dock) | `internal/tui/` | 6.2 |
| 6.5 | Message input bar | `internal/tui/` | 6.2 |
| 6.6 | System message display | `internal/tui/` | 6.3 |
| 6.7 | Key bindings + responsive layout | `internal/tui/` | 6.3 |

### Phase 7 — Integration

| # | Task | Depends on |
|---|------|------------|
| 7.1 | Wire TUI → Client logic → WebSocket → Server | All previous |
| 7.2 | Wire crypto into send/receive flow | 5.x, 7.1 |
| 7.3 | Integration tests | 7.1 |
| 7.4 | CLI flags (cobra) + config (viper) | 2.x |
| 7.5 | Retention sweep goroutine | 4.5 |

---

## Summary

| Severity | Count | Key Items |
|----------|-------|-----------|
| **Critical (P0)** | 6 | Pool panic, deadlock, server exits, reconnect loop, wrong column, inconsistent wire format |
| **Design** | 8 | Buffer race, connection dedup, unused return, no deadlines, no size limit, Pingdb path, discarded returns |
| **Dead code** | 11 | 2 dead packages, 6 unused functions, 2 unused types, 1 duplicate function |
| **Stubs** | 7 | 5 empty handlers + 1 empty error handler + 1 empty test |
| **Tests** | 3 | Package conflict, inverted args, empty stub |
