# TODO

## Release blockers

- [x] Identity persistence and returning-user login are broken again:
      `createNonce` relies on `PingUser`, but `PingUser` uses `Query`, which
      never returns `sql.ErrNoRows`; every successful lookup is treated as
      “user exists”, and the discarded rows leak. New users are not stored, so
      reconnects are treated as new users. (`src/server/router.go:33-55`,
      `src/server/db.go:102-115`). Failing tests: `TestHandshakeNewUserChallengeFlow`,
      `TestHandshakeBadSignatureRejected`, `TestHandshakeReturningUserChallengeFlow`.
- [x] Identity keys never persisted because `-ephemeral false` was parsed as a
      non-empty string. The CLI now parses a real bool and the session loader
      persists the identity key (`cmd/client/main.go:12-17`,
      `src/client/websocket.go:316-329`, `src/client/crypto_utils.go:134-151`).
- [x] The malformed-key `ed25519.Verify` panic is guarded by a 32-byte length
      check (`src/server/verification.go:83-89`). The separate unhandled-panic
      boundary for the hub remains open below.
- [x] `Hub.Listener` has no recovery boundary; a future panic in packet
      dispatch can terminate the sole routing goroutine for every client
      (`src/server/websocket.go:57-108`).
- [x] `NewDatabase` prepends `$HOME` to every location not beginning with `~`,
      including absolute paths, so the database is created outside the
      requested directory (`src/server/db.go:28-41`).
- [x] A new identity taking an existing nickname receives two challenges:
      the suffixed-name branch calls `createNonce` and then issues another
      nonce before returning. Both responses can complete the handshake twice
      (`src/server/router.go:98-128`). Failing test: `TestHandshakeSameNameDifferentKeyGetsSuffix`.

## UX

Not sure I want to do this. the whole point of an inactivity_timeout is to kick users once it expires.

- [ ] Idle users are disconnected after `InactivityTimeout`: transport-level
      pongs do not reset the per-read context, so the current server ping
      goroutine does not keep a silent client alive. Wire `PingPacket` as an
      application heartbeat and handle it server-side as a no-op
      (`src/server/websocket.go:126-163`, `src/shared/types.go:105-110`).

## Security

- [ ] End-to-end sender authentication is incomplete: `MessagePacket.Signature`
      is never populated or verified; directory/join packets distribute HPKE
      keys without identity-key proofs; and the client/server still use
      `ws://`/`ListenAndServe`. Bind HPKE keys to verified identities, sign and
      verify messages, and add authenticated TLS/wss. (`src/shared/types.go:39-50,87-98`,
      `src/client/websocket.go:279-288,335-339`, `src/server/router.go:149-170`,
      `src/server/websocket.go:218-237`, `src/server/tls.go:3-5,27-72`).
- [x] Challenge nonces are consumed before signature verification and are not
      bound to the connection, so a plaintext observer can burn or transplant
      a challenge. Verify first and atomically consume a nonce tied to the
      expected connection/identity (`src/server/verification.go:42-58,75-89`).
- [ ] Failed challenge attempts persist TOFU rows before any signature is
      verified, allowing an unauthenticated client to reserve nicknames/keys
      and fill the database. Persist only after successful authentication
      (`src/server/router.go:33-55,87-96,133-146`).
- [ ] No message replay protection: `SeenNonces` is never initialized or
      consulted, and a valid `MessagePacket` can be delivered repeatedly. Add
      a unique message id/nonce and reject repeats at the recipient
      (`src/client/types.go:25-35`, `src/shared/types.go:39-54`,
      `src/server/router.go:174-218`).
- [ ] A verified connection can send another handshake and overwrite its
      nickname/keys without clearing `Verified`, allowing authenticated state
      to be reused for a different identity. Reject renegotiation or reset
      verification before processing a new handshake
      (`src/server/router.go:58-85`).
- [ ] Refuse a second online connection for an already-authenticated nickname;
      `addClient` registers sockets before authentication, while the directory
      includes unverified empty entries and silently overwrites duplicate
      nicknames (`src/server/websocket.go:20-28`, `src/server/router.go:154-165`).

## Availability

- [ ] `Broadcast` holds `h.mu.RLock()` during each network write. A slow client
      can hold the lock for the 30-second write timeout and block connection
      registration/removal (`src/server/responses.go:49-61`). Snapshot clients
      under the lock, then write after unlocking.
- [x] Each websocket reader synchronously sends into the 64-entry hub channel;
      if the listener is blocked or the queue fills, all readers can block and
      routing stops (`src/server/types.go:39-47`, `src/server/websocket.go:148-164`).
