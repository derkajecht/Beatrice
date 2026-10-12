# TODO

Priority-ordered. P0 = do before any release; P1 = ship-blocking crypto integrity;
P2 = the safety net that lets P0/P1 stay fixed.

Derived from a full review of `go-rewrite` vs `main` (47 commits, ~4400 new lines).
Every line reference below was re-verified against the current tree.

---

## P0 — Authentication bypass

- [x] **Verified connections can impersonate any user.** `HandleHandshake`
      overwrites `c.Nickname`/`c.PubKey`/`c.HPKEPubKey`/`c.HPKEKem` but never
      resets `c.Verified = false`. `Verified` is set in exactly one place
      (`router.go:135`) and never cleared, and `Listener` has no gate preventing
      a second handshake. Attack: verify legitimately under your own key, then
      send `h` again with the victim's nickname and the victim's ed25519 public
      key (public data, no secret needed) and simply never answer the challenge.
      The connection now holds `Nickname=victim, Verified=true`, so
      `HandleMessage:174` relays as the victim, `HandlePresence:220` broadcasts
      as the victim, and unicast `:192` delivers the victim's inbound messages to
      the attacker. (`src/server/router.go:48-120,135,174,192,220`)
- [x] **Nonces are not bound to an identity.** `Issue` stores only
      `nonce -> expiry`; `CheckChallenge` verifies against the connection's
      *current, mutable* `c.PubKey` rather than the key the nonce was issued for.
      A nonce issued under key A can be answered after swapping the connection
      to key B — the challenge proves possession of *some* key, not the identity
      in use. This is the enabler for the bug above. (`src/server/verification.go:28-40,82-93`)
- [ ] **`Consume` and `BurnNonce` are two operations, not one atomic step.**
      Store `{nonce -> expectedPubKey, conn, expiry}` at issue time, verify
      against the stored pubkey, and make consume+burn a single atomic call.
      (`src/server/verification.go:43-63`)
- [x] **Directory publishes unverified connections.** `completeHandshake` builds
      `DirPacket.CurrentUsers` from *all* `h.clients` with no `Verified` filter;
      `addClient` registers every socket pre-auth. Connect, send `h` with a target
      nickname and your own HPKE key, never answer — every client that handshakes
      while you hold the socket open receives `victim -> your_key` in its
      directory. Join/presence/leave broadcasts also reach unauthenticated
      sockets. (`src/server/router.go:148-153`, `src/server/websocket.go:83-119,131-132`)
      Note: the comment at `websocket.go:131-132` ("only verified connections
      ever joined the directory") is false.
- [ ] **TOFU rows are persisted before any signature is verified**, with no
      `len(PubKey) == 32` check, for both new and suffixed users. Any
      unauthenticated connection can permanently squat nicknames with junk or
      empty keys and grow the DB without bound; real users are then forced onto
      `name-<hash>` suffixes forever. (`src/server/router.go:74-78,99-108`, `src/server/db.go:94-100`)

## P1 — Crypto integrity

- [ ] **TLS certificate verification is unconditionally disabled.**
      `InsecureSkipVerify: true` on every dial, while `StartClient` always uses
      `wss://`. Any on-path attacker terminates TLS, reads all metadata, and
      substitutes the HPKE public keys in the server-authoritative `d`/`j`
      packets — messages are then sealed to the attacker, defeating *all* E2E
      confidentiality. `wss://` currently buys nothing. Delete the field; gate any
      dev-mode bypass behind an explicit flag that warns loudly.
      (`src/client/websocket.go:29-31,385`)
- [ ] **AAD no longer binds sender/recipient.** Both seal and open use the
      constant `aad := []byte("additional data")`. `b13bc7a` commented out
      `shared.MessageAAD(u.Nickname, target)` / `MessageAAD(sender, recipient)`
      and replaced both with a literal; `MessageAAD` now has **zero call sites**,
      and `DecryptMessage`'s `sender`/`recipient` parameters appear only in its
      signature and doc comment. Because `MessagePacket.Signature` is commented
      out and HPKE runs in base mode, a malicious server can re-label any
      captured ciphertext with an arbitrary sender and the client decrypts and
      displays it. Comments at `crypto_utils.go:206-208` and `router.go:200-202`
      assert a binding that does not exist. (`src/client/crypto_utils.go:208-209,231,245`,
      `src/shared/types.go:63-65`)
- [ ] **No message authenticity.** HPKE base mode (`NewSender`/`Setup`), no
      signature on `MessagePacket`. Anyone holding a recipient's HPKE key —
      broadcast to every client in the directory — can forge messages to them.
      Challenge-response identity verification therefore contributes nothing to
      message authenticity. Use auth-mode HPKE, or sign a canonical
      domain-separated envelope with the identity key.
      (`src/client/crypto_utils.go:190,198`, `src/client/websocket.go:332`,
      `src/shared/types.go:52`)
- [ ] **The HPKE key is never bound to the verified identity.** The challenge
      signs only the nonce, so the key peers actually encrypt to is unauthenticated
      and a malicious server can substitute it with no signature failure. Sign
      `nonce || nickname || KEM || hpkePublicKey` and verify that transcript.
      (`src/server/verification.go:93`, `src/server/router.go:69-70,150-152`)
- [ ] **KDF and AEAD never cross the wire** — only KEM does. A same-KEM/
      different-KDF peer cannot be detected before `Open` fails; a mismatch
      degrades to silently dropped messages. Either hardcode one suite and delete
      the `NewCryptoSuite` params so a mismatch is inexpressible, or put a
      suite/version identifier on the wire and validate it at handshake.
      (`src/shared/types.go:15-18`, `src/client/crypto_utils.go:38-45,121-125`)
- [ ] **`DirPacket` carries one aggregate KEM written inside a map iteration**,
      so it is nondeterministic (map-order) last-writer-wins, and
      `CurrentUsers` loses per-user KEM. Carry `map[string]WireHPKEPubKey`
      and decode per entry. (`src/server/router.go:148-153`, `src/client/types.go:84-96`)

## P2 — Dead safety net (fix alongside P0/P1 so repairs stay pinned)

- [ ] **The entire test suite for both packages has not compiled for 4 commits.**
      `go build ./...` passes and `go test ./...` does not, which masks it:
      - `src/client/crypto_utils_test.go:66,78,81,204`
      - `src/server/handshake_test.go:83,211`
      - `src/server/unicast_test.go:166`
      Cause: `b13bc7a "feat: migrate chat encryption to CIRCL HPKE"` (2026-09-18)
      renamed `HPKEPubKey`→`EncodedPubKey`, `JoinPacket.PubKey`→`EncodedPubKey`,
      added KEM/KDF/AEAD params to `NewCryptoSuite` and a second arg to
      `ApplyDirPacket`, without updating a single test. Parent `df0ffe1` vets clean.
      Dead tests include `TestChallengeNonceReplayRejected`,
      `TestUnverifiedSenderRejected`, `TestHandshakeBadSignatureRejected` and
      `TestDecryptRejectsAADTampering` — the last of which **will still fail after
      the field renames are repaired**, because it asserts the P1 AAD binding that
      no longer exists. That is correct: it is your regression guard for that fix.
- [ ] **No CI anywhere** (`.github/workflows`, `.gitlab-ci.yml`, `.circleci` all
      absent). Add a pipeline running `go vet ./... && go test ./...`. `go build`
      alone is insufficient — it passed on this branch while the whole suite was red.

## P3 — Concurrency and crashes

- [ ] **`BurnNonce` mutates a mutex-protected map without the mutex.**
      `delete(m.nonces, nonce)` takes no lock while `Issue` and `cleanupNonces`
      both do. Unsynchronized concurrent map writes are undefined behavior and
      can throw a fatal `concurrent map writes` error, taking down the server.
      (`src/server/verification.go:58-63` vs `:28-40,65-77`)
- [ ] **`ServerClient` identity fields are written without `h.mu`** but read by
      other goroutines under `RLock` (which does not synchronize with unlocked
      writes). The disconnect defer can observe a stale `Verified`/`Nickname` and
      send the wrong leave broadcast; the directory can publish a torn field.
      (`src/server/router.go:67-70,135` vs `:148-153`, `src/server/websocket.go:133-134`)
- [ ] **`User.ConnectedUsers` reads are unlocked.** Writes are locked
      (`types.go:94-114`) but `BroadcastMessage` and `EncryptMessage` iterate the
      live map with no lock. Pressing Enter while a join/leave/directory packet
      arrives is `concurrent map iteration and map write` — a *fatal* runtime
      throw, not just a race warning, and a peer can trigger it by
      reconnect-spamming. Note there is a commented-out `SnapshotUsers` at
      `types.go:119-123` — the fix was started and abandoned. Expose only
      locked snapshot accessors; never hand out the live map.
      (`src/client/websocket.go:295`, `src/client/crypto_utils.go:160,169`)
- [ ] **`User.Nickname` is written without a lock** (`websocket.go:167`) while the
      TUI goroutine reads it (`:305,327,356`, `crypto_utils.go:166`). Can cause
      encrypting to yourself or misattributing the sender field. Add
      `GetNickname`/`SetNickname` under `mu`. (`src/client/websocket.go:167,305,327,356`)
- [ ] **Unbounded goroutine per incoming frame.** The read loop spawns a goroutine
      doing a blocking `h.broadcastChn <- ...` on a 64-slot channel with no
      `ctx.Done()` select. If the listener stalls or exits, goroutines accumulate
      and leak past shutdown; per-connection ordering is also lost. Use a bounded
      per-connection queue with an explicit drop/backpressure policy.
      (`src/server/websocket.go:176-178`, `src/server/types.go:44,53`)
- [ ] **`recover()` boundary in `Listener` does not cover the whole dispatch.**
      The panic handler wraps the `switch`, but a panic in `conn.close`/deferred
      teardown or in a handler's own deferred path still escapes the sole routing
      goroutine for every client. Verify the boundary covers the full message
      lifecycle, not just the dispatch body. (`src/server/websocket.go:83-119`)

## P4 — Availability / DoS

- [ ] **Socket writes run synchronously in the single hub goroutine**, each with a
      hardcoded 30s deadline from `context.Background()`. One verified recipient
      that stops reading stalls routing for all users; repeated messages give a
      sustained DoS. `Broadcast` also holds `h.mu.RLock()` across the network write,
      blocking registration/removal/lookups. Move writes to a per-connection writer
      goroutine; snapshot the client slice under the lock, release, then enqueue.
      (`src/server/router.go:204`, `src/server/responses.go:21-22,49-61`)
- [ ] **No connection cap, handshake/message rate limit, nonce-map cap, or auth
      deadline.** Every connection joins the hub pre-auth and every handshake
      mints a nonce plus possibly a DB row, so a few sockets inflate all three
      without bound. Add token-bucket limits per connection/IP and drop connections
      that never verify within N seconds. (`src/server/websocket.go:185-194`)
- [ ] **`UsernameSanitizer` logs once per invalid rune and rebuilds the string per
      rejected rune.** Only a *minimum* nickname length is enforced, so a ~1 MiB
      nickname of punctuation emits ~1M synchronous `slog.Warn` writes inside the
      single listener goroutine (plus O(n²) work). Enforce a max length (e.g. 32)
      first, sanitize in one pass, and log at most once. HPKE KEM/key bytes are
      accepted unvalidated (up to the 1 MiB frame limit) and amplified to every
      client. (`src/server/validation.go:19-28`, `src/server/router.go:55-58,69-70`)
- [ ] **Client log-channel backpressure stalls the read loop.** `ChannelWriter.Write`
      blocks on `logCh` (cap 100) while `readLoop` emits an `slog.Error` per
      malformed packet and the TUI retains every log line forever. A server
      flooding malformed packets fills the channel and stops socket reads. Make the
      writer non-blocking (drop-oldest + counter), rate-limit per-packet error
      logs, and cap retention. (`src/client/logger.go:11-21`,
      `src/client/websocket.go:155,163,196,214,224`)

## P5 — Client correctness

- [ ] **Terminal escape / ANSI injection from peer-controlled content.** Decrypted
      plaintext, sender nicknames, leave text and server `"e"` error messages are
      rendered verbatim through lipgloss, which strips nothing. A peer can emit
      `\x1b[2J`, OSC 52 (clipboard write), OSC 8 (spoofed hyperlink) or fake UI
      chrome. Sanitize ESC and C0/C1 at the packet boundary in `handlePacket` —
      not per view — so no future view can forget. (Note `validateColor` in
      `config.go` already blocks this via config; the packet path does not.)
      (`src/client/tui/chatView.go:132-136,146,271`, `src/client/tui/main.go:460,474`,
      `src/client/tui/header.go:62,71`, `src/client/tui/sidebar.go:122`)
- [ ] **No message replay protection.** No message id/nonce on `MessagePacket`;
      `DecryptMessage` accepts any ciphertext that opens and the TUI appends it.
      `SeenNonces` is declared but never initialized or consulted, and `mp.Time` is
      displayed but never validated. Add a random msgID plus a recipient-side TTL
      replay cache. (`src/client/crypto_utils.go:231`, `src/client/types.go:34-36`,
      `src/client/websocket.go:218-237`)
- [ ] **Partial encryption/send failures are invisible; the TUI reports success.**
      `EncryptMessage` returns the successful subset plus an error;
      `BroadcastMessage` only logs a warning if at least one send worked and returns
      nil; `submit` clears the composer and locally echoes. Users believe a message
      reached everyone when it reached a subset. Return a structured
      delivered/failed result and surface "delivered to N of M".
      (`src/client/crypto_utils.go:221-224`, `src/client/websocket.go:311-320,344-347`,
      `src/client/tui/main.go:365-380`)
- [ ] **Decrypted messages are silently dropped when `TuiChan` is full** — the
      `default:` branch logs and discards, including fully decrypted `"m"` packets,
      with no gap indication shown to the user. (`src/client/websocket.go:248-257`)
- [ ] **Network I/O runs synchronously on the Bubble Tea update goroutine.**
      `submit` calls `send(...)` inline (presence correctly uses a `tea.Cmd`); each
      recipient gets a fresh 30s `context.Background()` write, so the UI can freeze
      for N×30s and quit/cannot cancel in-flight writes. Make it a `tea.Cmd` and
      thread the root context through. (`src/client/tui/main.go:365`,
      `src/client/websocket.go:284-286`)
- [ ] **Unbounded chat history with a full O(history) re-render per message.**
      `m.messages` is append-only and `rebuild()` re-wraps the entire history on
      every message, resize and nickname change; a single 1 MiB message expands to
      hundreds of thousands of lines. Use a bounded ring buffer and/or incremental
      viewport updates with a per-message render cap. (`src/client/tui/chatView.go:47-48,76-105`)
- [ ] **Identity key file write is non-atomic and silently regenerates on
      corruption.** Loading accepts any 64-byte file without checking that the
      embedded public half matches the derived key; a crash-truncated file makes
      `len != 64`, so the next start silently creates a **new** identity and
      overwrites — irreversible identity loss. Write temp+fsync+rename+chmod 0600;
      verify the pub half on load; treat short/malformed files as a hard error.
      (`src/client/crypto_utils.go:98-114`)
- [ ] **`EncodePubKey` error is discarded before the handshake send**, so a failed
      encode transmits a zero-valued HPKE key and the client stays connected but
      unencryptable. Check the error and abort to the reconnect loop.
      (`src/client/websocket.go:91-96`)
- [ ] **`ApplyDirPacket` aborts the whole snapshot on one bad key** (`return`, no
      log), leaving `ConnectedUsers` stale: departed users remain valid encryption
      targets and new joiners are missing until the next packet. `continue` past bad
      entries and apply the good subset. (`src/client/types.go:89-91`)
- [ ] **Sidebar never removes users absent from a refreshed directory** — the `"d"`
      handler only adds. Rebuild from the snapshot so the roster does not lie after
      a reconnect. (`src/client/tui/main.go:396-408`)
- [ ] **Challenge signing is a bare-nonce signature oracle.** The client signs
      `[]byte(cp.PendingAuth)` on any `"c"` packet, ignoring `cp.Nickname`,
      `cp.PubKey`, `cp.IsNew`, and whether a handshake was even pending. No domain
      separation. Since the code anticipates reusing this key for message signatures,
      a chosen challenge could become a valid signature over a transplantable
      "message". Only respond while a handshake is pending, verify the nickname,
      and sign `"beatrice/auth/v1" || nickname || nonce`.
      (`src/client/websocket.go:171-186`)
- [ ] **`CryptoPacket.PrivKey` lacks `json:"-"`** while `IdentityPrivKey` correctly
      has it and `User` is fully tagged as serializable. Nothing marshals it today,
      but any future `slog.Any("user", u)` or debug dump would serialize key
      material. Add `json:"-"` and strip JSON tags from crypto structs.
      (`src/client/types.go:30-33`)

## P6 — Correctness and protocol gaps

- [ ] **No message persistence or offline delivery**, contrary to blueprint phases 3
      and 5. The `messages` table is created but never inserted into or read; messages
      for offline recipients are dropped and reported as `err_unknown_recipient`,
      which also doubles as an online-status oracle. Implement encrypted-blob
      persistence or remove the dead table and return an explicit
      "recipient offline, not delivered". (`src/server/router.go:191-198`,
      `src/server/db.go:68-78`, `blueprint.md:30-36,56-62`)
- [ ] **Absolute database paths are incorrectly `$HOME/`-prefixed.** Only a leading
      `~` should expand; anything else is used as-is. `-dbLocation /var/lib/beatrice`
      currently writes to `$HOME//var/lib/beatrice`. Resolve once, in one place
      (`Pingdb` also re-expands `~` that `NewDatabase` already expanded).
      (`src/server/db.go:28-47,61-67`)
- [ ] **Duplicate verified nicknames are allowed** and `getClientByNickname` returns
      a map-order-random one; one device disconnecting broadcasts `l` while the other
      is still online. Track live connection counts per nickname before broadcasting
      leave, and define a duplicate-nickname policy before publishing the directory.
      (`src/server/websocket.go:44-53,129-137`)
- [ ] **No protocol version anywhere**, and wire field JSON tags are asymmetric
      (`HandshakePacket.EncodedPubKey` and `DirPacket.EncodedPubKey` untagged while
      `JoinPacket` uses `json:"k"`). `b13bc7a` silently replaced a flat `"hk"` field
      with a nested object, so an older/non-Go client ignores it and ends up with an
      empty directory and no error. Add explicit tags plus a version field, and
      reject unknown versions. (`src/shared/types.go:15-18,27,34,92,97,103,117-119`)
- [ ] **Dead or misleading wire fields:** `LeavePacket.PubKey` (sent, never read, and
      unusable without a KEM id), `ChallengePacket.PubKey` (never read by the client),
      `ChallengePacket.IsNew` (hardcoded `false` server-side), and the `shared.PubKey`
      type (referenced nowhere). Delete or wire them deliberately.
      (`src/shared/types.go:34,103,117-119`, `src/server/router.go:25,134`)
- [ ] **Stale comments that will mislead the next audit** — fix the text, not the code:
      `verification.go:43-44` claims burning happens before verification (it happens
      after, which is correct); `websocket.go:131-132` claims only verified clients
      join the directory (false); `crypto_utils.go:206-208` and `router.go:200-202`
      claim recipients verify AAD against authoritative names (no call sites);
      `shared/types.go:56-57,110-112` and `websocket.go:163-165` heartbeat TODOs.
- [ ] **Hardcoded `~/` certificate paths** in `websocket.go:21-24` mean
      `StartServer` cannot run on another machine without editing source. Move to
      flags/env with the current values as defaults. `cleanupNonces` goroutine and
      ticker are also never stopped (per-Hub leak). `BurnNonce`'s error is ignored
      at the call site, and broadcast write failures are logged without evicting the
      client. (`src/server/websocket.go:21-24`, `src/server/verification.go:23,65-77`)
- [ ] **`beginUserRegistration` errors are discarded** (client hangs unverified with
      no log) and its `exists` parameter is unused. On the `!cexists` suffix path,
      `router.go:99-117` issues **two** challenge nonces — the first leaks until
      expiry and both responses can complete the handshake. Issue exactly one.
      (`src/server/router.go:78,80,99-117`)
- [ ] **Idle-timeout behavior needs a decision.** Transport pongs do not reset the
      per-read context, so the server survives idle *only* because Go clients send an
      application `"i"` heartbeat every 30s; non-Go or older clients are dropped at
      `InactivityTimeout`. Either track read/pong activity and reset the deadline, or
      document the app heartbeat as mandatory. (`src/server/websocket.go:139-169`)

## P7 — Repository hygiene

- [ ] **Compiled binary and SQLite DB are tracked in git:** `bin/client` (15 KB) and
      `src/archive/beatrice_server.db`. Untrack and gitignore them.
- [ ] `TestNewCryptoSuite` is a zero-case, zero-arg stub that asserts nothing and
      uses the pre-`b13bc7a` API. Replace with cases asserting the exact KEM/KDF/AEAD
      triple, or delete. `HasEmptyArgs` has no test and treats whitespace-only as valid.
      (`src/client/crypto_utils_test.go:195-209`, `src/shared/sharedvalidation_test.go:5-21`)

---

## Stale — marked `[x]` previously but reverified as STILL BROKEN

These were checked off in earlier TODO revisions but the underlying code is
unchanged or only half-fixed. Un-tick them.

- [ ] **`NewDatabase` prepends `$HOME` to every non-`~` location**, including absolute
      paths. The code is unchanged and now carries a TODO admitting it at
      `db.go:13-15`; the `else` branch still does `home + "/" + cfg.Location`.
      (`src/server/db.go:28-47`) — duplicate of P6 above.
- [ ] **A new identity taking an existing nickname still receives two challenges.**
      `router.go:99-117` issues a nonce inside `if !cexists` and sends it, then falls
      through to issue a *second* nonce and return. Both responses can complete the
      handshake. (`src/server/router.go:95-120`) — see P6.
- [ ] **Nonces are now burned *after* verification (that part is genuinely fixed), but
      they are still not bound to the connection or identity, and consume+burn are
      still two separate non-atomic operations.** Only half the item was addressed.
      (`src/server/verification.go:43-63`, `src/server/router.go:122-136`) — see P0.
- [ ] **The 64-entry hub channel can still stall routing.** The synchronous send was
      wrapped in `go func(){...}()`, which trades a deadlock for an unbounded goroutine
      leak. The TODO at `websocket.go:173-175` still describes the original problem.
      (`src/server/websocket.go:176-178`) — see P3.

Genuinely fixed and correctly ticked: the `PingUser`/`Query` `ErrNoRows` identity bug,
the `-ephemeral` bool parse, the `ed25519.Verify` panic guard, and the `Listener`
panic-recovery boundary.

---

## Verified correct — do NOT "fix"

Recorded so these do not get "cleaned up" later.

- **Concurrent websocket writes are safe.** `github.com/coder/websocket` serializes
  writes internally. The P4 finding is about blocking/head-of-line, not
  unsynchronized writes. Do not add a write mutex.
- **Burn-after-verify is the right order** — burning only on success stops a
  nonce-guesser from invalidating a legitimate pending challenge. The TODO is wrong,
  not the code.
- **HPKE nonces are correct.** `Setup(rand.Reader)` randomized base mode; no counter,
  no reuse. Do not add an IV.
- **`info = []byte("beatrice")` is symmetric** between `NewSender` and `NewReceiver`;
  the server never performs HPKE, so there is no cross-side mismatch to reconcile.
- **Server-side sender spoofing is correctly prevented** — `router.go:202,237`
  overwrite wire `Sender`/`Nickname` with the connection identity.
- **SQL is parameterized** and `ErrNoRows` handled; no injection. Public-key
  `bytes.Equal` needs no timing-safe compare (public keys are not secrets).
- **No plaintext is stored or relayed by the server.** `SetReadLimit(1<<20)` is set
  before any read.
- **No plaintext fallback on encryption failure** — `EncryptMessage` drops failing
  targets and errors; it never sends unencrypted content.
- **The identity private key is never logged, rendered, or sent.**
- **Config file permissions are correct** (`0700` dir / `0600` file), and
  `KnownFields(true)` rejects unknown keys.
- **`ChannelWriter.Write` copying the buffer is necessary** — slog reuses the slice.
- **Reconnect is not a hot loop** — exponential backoff with jitter, no double-send,
  no goroutine leak.
