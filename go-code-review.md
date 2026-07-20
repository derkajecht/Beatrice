# Go Rewrite — Code Review

## Critical Bugs

### 1. `sync.Pool` type mismatch causes panic

**File:** `internal/listener/listener.go:34,179`

Pool's `New` factory creates `[]byte` (a slice header), but `Put(&buf)` passes `*[]byte` (pointer to slice header). After one cycle, `Get().([]byte)` receives a `*[]byte` and the type assertion panics.

**Fix:** Change `bufferPool.Put(&buf)` to `bufferPool.Put(buf)`.

---

### 2. Deadlock: `HandleHandshake` + `GetUserList` both lock `ChatRoom`

**Files:** `internal/utils/router.go:19`, `internal/models/packets.go:24`

`HandleHandshake` acquires `types.ChatRoom.Lock()` then calls `models.GetUserList()`, which calls `r.Lock()` on the same `*sync.Mutex`. Go's `sync.Mutex` is **not reentrant** — this deadlocks on the second concurrent connection.

**Fix:** Either:
- Drop the lock in `HandleHandshake` before calling `GetUserList` (add/remove the user while locked, then unlock before querying), or
- Add a `GetUserListLocked` variant that assumes the caller holds the lock.

---

### 3. Server exits immediately at startup

**File:** `cmd/server/main.go:14-34`

`api.NewServer(host, port)` is launched in a goroutine but `main()` has no blocking mechanism — it returns immediately after setting up the DB. The program exits before the TCP listener can accept any connections.

Additionally, `defer db.Close()` never runs because `main()` returns, not the program exiting via `os.Exit`. Actually `main()` returning does run defers, but the goroutine processing never gets a chance to start.

**Fix:** Add `select{}` at the end of `main()` or use `sync.WaitGroup` to block indefinitely.

---

### 4. `NewClient` infinite reconnect loop

**File:** `internal/api/connection.go:73-82`

The `for {}` loop dials, handles (which returns when the connection closes), then immediately dials again in a tight loop with no back-off, stop condition, or cancellation mechanism. It will spam connections on any transient failure.

**Fix:** Remove the `for {}` loop — `NewClient` should establish a single connection, or at minimum add a back-off/sleep between reconnect attempts.

---

### 5. `IsValidUsername` queries wrong column

**File:** `internal/validation/validation.go:23`

Query: `SELECT id FROM users WHERE username = ?`

Schema (`internal/storage/db.go:50`): column is `nickname`, not `username`.

**Fix:** Change `username` to `nickname` in the query.

---

### 6. Inconsistent error packet wire format

**Files:** `internal/utils/responses.go:15-36` vs `internal/utils/responses.go:40-77`

`SendError` writes a bare `{"m": "..."}` to the wire. `SendPacketToClient` wraps in an envelope: `{"t": "e", "m": {...}}`. Clients see two different wire formats for the same logical packet type depending on which code path sent the error.

**Fix:** Send all errors through `SendPacketToClient` with an envelope, or make `SendError` wrap in an envelope consistently.

---

## Design Issues

### 7. `bufferPool` buffer reuse race

**File:** `internal/listener/listener.go:34,178-179`

`ClearBuffer` zeros the buffer byte-by-byte after `Put` into pool. Another goroutine could `Get` the buffer while it's being cleared, reading zeros mid-operation. Also the byte-by-byte zeroing is unnecessary in Go — the GC handles memory.

**Fix:** Remove `ClearBuffer` call entirely, or zero before `Put` (not after).

---

### 8. `HandleHandshake` connection dedup by pointer

**File:** `internal/utils/router.go:23`

```go
if types.ChatRoom.Clients[conn] != nil {
```

Since `net.Conn` is an interface, the map key is the interface value (pointer under the hood). A reconnecting client gets a new `net.Conn` value that will never match an existing entry. The "already connected" check can never fire on reconnect.

**Fix:** Check by nickname instead of connection pointer.

---

### 9. `NewDatabase` returns unused path

**File:** `internal/storage/db.go:16`

Returns `(*sql.DB, error, string)` — the third return value (file path) is ignored by all callers. Unusual Go idiom; suggests the caller should construct the path and pass it in.

**Fix:** Remove the third return value, or accept the path as a parameter.

---

### 10. No read/write deadlines on connections

**File:** `internal/listener/listener.go`

`conn.Read(buf)` blocks indefinitely with no timeout. A hung client connection will pin a goroutine forever. This is necessary for detecting dead peers until WebSocket ping/pong is implemented.

**Fix:** Add `conn.SetReadDeadline(time.Now().Add(readTimeout))` before each read.

---

### 11. No payload size limit

**File:** `internal/listener/listener.go:38`

The 4096-byte buffer is read into, but if the client sends more than 4096 bytes, the read is truncated silently, leading to JSON decode errors at best, or protocol desync at worst. If they send less, the buffer contains stale zeros from the pool.

**Fix:** Use `io.ReadFull` or a `bufio.Scanner` with `MaxScanTokenSize`, or length-prefix messages.

---

### 12. Module path casing

**File:** `go.mod:1`

`github.com/derkajecht/Beatrice` — Capital `B` in `Beatrice`. On case-sensitive filesystems (Linux), importing this as a dependency requires exact casing. Minor issue for a standalone project but worth noting.

---

## Wire Protocol / Compatibility

### 13. Raw TCP vs WebSocket

Python server uses WebSockets (`websockets` library via FastAPI). Go server uses raw TCP (`net.Listen`/`net.Dial`). These are **wire-incompatible** — they cannot communicate with each other.

### 14. Packet envelope mismatch

Python sends flat JSON packets (e.g. `{"t": "M", "r": "Bob", ...}`). Go expects a two-layer envelope: `{"t": "m", "m": {"r": "Bob", ...}}`. Different packet format entirely.

### 15. Port default mismatch

| Component | Default Port |
|-----------|-------------|
| Go server | `8080` |
| Go client | `8080` |
| Python server | `55556` |
| Python client | `55556` |

If the plan is to have a single server that both clients can talk to, the port needs to match.

---

## Tests

### 16. `TestNewServer_EmptyPort` has inverted arguments

**File:** `tests/connections_test.go:10`

```go
err := api.NewServer("", "localhost")
```

The function signature is `NewServer(host, port string)`. This passes `host=""`, `port="localhost"`. The function validates port is empty, but the test name says "EmptyPort" yet passes an empty host.

**Fix:** Swap arguments to `NewServer("localhost", "")`.

### 17. `user_test.go` is an empty stub

**File:** `tests/user_test.go`

Contains only a TODO comment and no actual test logic.

---

## Summary

| Severity | Count | Key Items |
|----------|-------|-----------|
| **Critical** | 6 | Pool panic, deadlock, exits immediately, reconnect loop, wrong column, inconsistent wire format |
| **Design** | 6 | Race condition, connection dedup, unused return, no deadlines, no size limit, module casing |
| **Compat** | 3 | TCP vs WS, envelope mismatch, port mismatch |
| **Tests** | 2 | Inverted args, empty stub |
