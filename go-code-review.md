# Go Rewrite — Code Review & Action Plan

> **Context:** The Python implementation will be deprecated. The Go rewrite is a standalone app
> with its own wire protocol, crypto, and architecture. No backward compatibility required.

> **Session (2026-07-27):** Phase 0 + most of Phase 1 done. Listener split into
> `HandleConnection` + `Dispatch`. Transport decided: **`coder/websocket`**.

---

## Table of Contents

1. [Critical Bugs (P0 — blocking execution)](#1-critical-bugs)
2. [Design Issues (P2 — structural)](#2-design-issues)
3. [Redundant / Dead Code](#3-redundant--dead-code)
4. [Test Issues](#4-test-issues)
5. [Architecture & Wire Protocol](#5-architecture--wire-protocol)
6. [Execution Plan](#6-execution-plan)

---

## Progress snapshot

| Phase | Status | Notes |
|-------|--------|-------|
| **0 — P0 bugs** | **Done** | All 0.1–0.8 addressed (tests removed rather than fixed) |
| **1 — Dead code** | **Mostly done** | Packages deleted; `Pingdb(path)` still open; some symbols kept on purpose |
| **2 — Transport** | **Decision made** | `coder/websocket` — swap not wired yet (raw TCP + leftover gorilla import) |
| **3–7** | Pending | Handlers, CRUD, crypto, TUI, integration |

---

## 1. Critical Bugs

### 1.1 `sync.Pool` type mismatch causes panic — **FIXED**

**File:** `internal/server/listener/listener.go`

Was `Put(&buf)` (`*[]byte`); now `Put(buf)` (`[]byte`). Matches pool `New` factory.

---

### 1.2 Deadlock: `HandleHandshake` + `GetUserList` both lock `ChatRoom` — **FIXED**

**Files:** `internal/server/router/router.go`, `internal/server/models/packets.go`

Outer `ChatRoom.RLock()` removed from `HandleHandshake`. Locking lives on `Room` methods (`GetClient` / `AddClient` / `GetUserList`) — no read→write upgrade.

---

### 1.3 Server exits immediately at startup — **FIXED**

**File:** `cmd/server/main.go`

`RunServer` opens DB, `defer db.Close()`, then calls `api.NewServer` **synchronously** (Accept loop blocks). `main` calls `RunServer` directly — no fire-and-forget goroutine.

---

### 1.4 `NewClient` infinite reconnect loop — **FIXED**

**File:** `internal/api/connection.go`

Single `net.Dial` + synchronous `HandleConnection(conn)`. No `for {}` reconnect spam. Client process stays alive until connection ends.

---

### 1.5 `IsValidUsername` queries wrong column — **FIXED**

**File:** `internal/server/validation/validation.go`

Query uses `nickname` (matches schema).

---

### 1.6 Inconsistent error packet wire format — **FIXED**

**File:** `internal/shared/utils/responses.go`

`SendError` removed. Errors go through `SendPacketToClient` envelope (`{"t":"e","m":{...}}`). `DisconnectAndQuit` uses type `"q"` — document or fold into `"e"` later.

---

## 2. Design Issues

### 2.1 Buffer pool reuse race — **partially addressed**

`clear(buf)` before `Put` is correct ownership. Still open: early-return paths must always `Put`; `defer Put` **inside the read loop** stacks defers until `HandleConnection` returns — move Put to end of iteration (no loop-scoped defer). Pool likely drops when Phase 2 switches to WS framing.

### 2.2 Connection dedup by net.Conn pointer — **OPEN**

Still keyed by `net.Conn`. Fix with nickname map / ConnectionManager (Phase 2.3).

### 2.3 `NewDatabase` returns unused path — **OPEN**

Third return still unused in meaningful way (`ServerStruct` still discarded — see 2.8).

### 2.4 No read/write deadlines — **OPEN → Phase 2.4**

WS ping/pong + deadlines via `coder/websocket`.

### 2.5 No payload size limit — **OPEN → Phase 2**

WS framing replaces fixed 4096 TCP buffer. Set max message size on conn.

### 2.6 `Pingdb()` ignores configured database path — **OPEN** (Phase 1.5)

Still uses `config.NewDatabaseInfo()` default. Needs path/`*sql.DB` param.

### 2.7 `NewUserSession()` return values discarded — **OPEN**

Still `_, _, _, err := crypto.NewUserSession()`. Wire when TUI/crypto path lands.

### 2.8 `ServerStruct()` return value discarded — **OPEN**

Still discarded. Wire or delete with ConnectionManager work.

---

## 3. Redundant / Dead Code

### 3.1 Entire packages that are dead — **DELETED**

| Package | Status |
|---------|--------|
| `internal/server/errors/` | **Deleted** |
| `internal/server/auth/` | **Deleted** |

### 3.2 Duplicated `UnmarshalPacket` — **RESOLVED**

Kept `validation.UnmarshalPacket` (callers updated). Removed helpers copy + commented `ClearBuffer`. Orphaned `logger.main` removed. `conversion.go` deleted.

### 3.3 Unused functions — **mixed**

| Function | Status |
|----------|--------|
| `types.ConvertPacketToMap` | **Deleted** (file gone) |
| `models.NewClientPacket` | **Keep** — planned post-handshake / client path |
| `storage.StoreUser` | **Keep** — Phase 4 CRUD |
| `logger.LoggerSetup` / `ChannelWriter` | Keep for TUI logging |
| `types.SuccessPacket` | Review later |
| `sharedvalidation.IsEmpty` | Used / shared — keep |

### 3.4 Stub router handlers — **still stubs** (Phase 3)

`HandleMessage`, `HandleJoin`, `HandleLeave`, `HandleDir`, `HandleChallenge`, partial `HandleError`.

### 3.5 Listener split (tonight)

`HandleConnection` = read / pool / unmarshal envelope → `Dispatch(conn, gen)`.
`Dispatch` = switch on `gen.Type`, `json.Unmarshal(gen.Message, &typed)`, call `router.Handle*`.

---

## 4. Test Issues — **CLEARED BY DELETION**

`tests/connections_test.go` and `tests/user_test.go` removed (package conflict + inverted args + empty stub). Re-add under Phase 7.3 / package-local `_test.go` files.

---

## 5. Architecture & Wire Protocol

### Key decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Transport | **`coder/websocket`** (decided) | Framing + ping/pong; prefer over gorilla (both briefly in `go.mod` — drop gorilla) |
| Crypto | HPKE (MLKEM768X25519 + HKDF-SHA512 + AES-256-GCM) | Already written, post-quantum, modern |
| Auth | Challenge-response over the single connection | No HTTP REST layer needed |
| History sync | Over the main connection on reconnect | No HTTP GET /messages/history needed |
| Packet format | Nested envelope `{"t":"...","m":{...}}` | Unified via `SendPacketToClient` + `GeneralPacket` |

### Current protocol state

| Packet Type | Direction | Status |
|-------------|-----------|--------|
| Handshake (`h`) | Client → Server | Partially implemented |
| Message (`m`) | Bidirectional | **Stub** |
| Join (`j`) | Server → All | **Stub** |
| Leave (`l`) | Server → All | **Stub** |
| Directory (`d`) | Server → Client | Sent (in handshake) |
| Error (`e`) | Server → Client | Envelope unified |
| Challenge (`c`) | Server → Client | **Stub** |
| Quit (`q`) | Server → Client | Used by `DisconnectAndQuit` — confirm in protocol |

### Target architecture

```
                           ┌──────────────┐
                           │   cmd/server  │
                           │   main.go     │
                           └──────┬───────┘
                                  │
                     ┌────────────┴────────────┐
                     │      internal/api        │
                     │  NewServer / NewClient   │
                     │  (coder/websocket)       │
                     └────────────┬────────────┘
                                  │
               ┌──────────────────┼──────────────────┐
               │                  │                    │
      ┌────────┴────────┐ ┌──────┴───────┐ ┌─────────┴─────────┐
      │internal/server/ │ │internal/server│ │ internal/server/  │
      │   listener/     │ │   router/     │ │   storage/        │
      │ HandleConnection│ │ Handle* fns   │ │ db.go (SQLite)    │
      │ + Dispatch (WS) │ │               │ │ CRUD ops          │
      └──────┬──────────┘ └──────┬───────┘ └───────────────────┘
             │                   │
             └───────┬───────────┘
                     │
         ┌───────────┴───────────┐
         │    internal/shared/    │
         │  types/ utils/        │
         │  sharedvalidation/    │
         └───────────┬───────────┘
                     │
      ┌──────────────┼──────────────┐
      │              │              │
┌─────┴──────┐ ┌─────┴──────┐ ┌────┴──────────┐
│internal/   │ │internal/    │ │ internal/tui   │
│client/     │ │server/      │ │ (planned)      │
│crypto/     │ │validation/  │ │ Bubble Tea app │
│logger/     │ │config/ etc  │ └───────────────┘
└────────────┘ └────────────┘
```

---

## 6. Execution Plan

### Phase 0 — Fix P0 Bugs — **DONE**

| # | Task | Status |
|---|------|--------|
| 0.1 | Fix `sync.Pool` Put type | Done |
| 0.2 | Fix deadlock in HandleHandshake | Done |
| 0.3 | Keep server alive (sync `NewServer`) | Done |
| 0.4 | Remove infinite reconnect loop | Done |
| 0.5 | Fix DB column name | Done |
| 0.6 | Unify error wire format | Done |
| 0.7 / 0.8 | Test package + args | Done (deleted `tests/`) |

### Phase 1 — Clean up dead code — **MOSTLY DONE**

| # | Task | Status |
|---|------|--------|
| 1.1 | Delete `internal/server/errors/` | Done |
| 1.2 | Delete `internal/server/auth/` | Done |
| 1.3 | Deduplicate `UnmarshalPacket` | Done (kept validation) |
| 1.4 | Delete unused types/functions | Partial — kept intentional stubs |
| 1.5 | Fix `Pingdb()` to accept path param | **OPEN** |
| 1.6 | Remove unused imports after cleanup | Done enough |

### Phase 2 — Wire protocol + `coder/websocket` — **NEXT**

| # | Task | Files | Depends on |
|---|------|-------|------------|
| 2.1 | Choose transport | — | **Done → `coder/websocket`** |
| 2.2 | Swap listener + api to WS; drop gorilla | `listener.go`, `connection.go`, `go.mod` | 2.1 |
| 2.3 | ConnectionManager (`map[nickname]conn`) | `internal/connection/` or types | 2.2 |
| 2.4 | Read deadlines / ping-pong / max msg size | listener | 2.2 |
| 2.5 | Keep `Dispatch` on top of WS read loop | listener | 2.2 |

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

| # | Task | Depends on |
|---|------|------------|
| 4.1–4.5 | SaveUser / GetUser / SaveMessage / GetMessages / DeleteOldMessages | **1.5** |

### Phase 5 — Crypto completion

Key persistence, HPKE round-trip, sign/verify, nonce cache, padding.

### Phase 6 — TUI (Bubble Tea)

Login, chat log, users panel, input, system msgs, key bindings.

### Phase 7 — Integration

Wire TUI → client → WS → server; crypto; tests; cobra/viper; retention sweep.

---

## Summary

| Severity | Open | Resolved tonight / prior |
|----------|------|---------------------------|
| **Critical (P0)** | 0 | 6 — pool, deadlock, server lifecycle, reconnect, column, error wire |
| **Design** | ~6 | buffer ownership improved; WS will close framing/deadline items |
| **Dead code** | low | 2 packages gone; conversion + helpers dup gone |
| **Stubs** | 6 handlers | Phase 3 |
| **Tests** | none | deleted — rebuild later |
| **Next** | Phase 2.2 | Wire `coder/websocket`, remove gorilla, keep `Dispatch` |
