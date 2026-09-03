package server

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/derkajecht/Beatrice/src/shared"
)

func newTestHub(t *testing.T) (*Hub, context.Context, context.CancelFunc) {
	dbInfo := TestDB(t, "test.db", t.TempDir())
	hub := NewHub()
	hub.db = dbInfo
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Listener(ctx)
	t.Cleanup(func() {
		cancel()
		dbInfo.DB.Close()
	})
	return hub, ctx, cancel
}

func dialTestServer(t *testing.T, ts *httptest.Server) *websocket.Conn {
	t.Helper()
	url := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	return conn
}

func mustMarshal(t *testing.T, v any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	return data
}

func readGeneralPacket(t *testing.T, conn *websocket.Conn) shared.GeneralPacket {
	t.Helper()
	readCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var gp shared.GeneralPacket
	if err := wsjson.Read(readCtx, conn, &gp); err != nil {
		t.Fatalf("read failed: %v", err)
	}
	return gp
}

// newIdentity generates a fresh real ed25519 keypair for a test client.
func newIdentity(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate identity key: %v", err)
	}
	return pub, priv
}

// sendHandshake writes a handshake packet presenting the given identity and
// HPKE public keys, exactly like the real client.
func sendHandshake(t *testing.T, conn *websocket.Conn, nickname string, pub ed25519.PublicKey, hpke []byte) {
	t.Helper()
	writeGeneralPacket(t, conn, shared.GeneralPacket{
		Type: "h",
		Message: mustMarshal(t, shared.HandshakePacket{
			Nickname:   nickname,
			PubKey:     pub,
			HPKEPubKey: hpke,
		}),
	})
}

// expectChallenge reads the next packet and asserts it is a challenge,
// returning the parsed ChallengePacket.
func expectChallenge(t *testing.T, conn *websocket.Conn) shared.ChallengePacket {
	t.Helper()
	reply := readGeneralPacket(t, conn)
	if reply.Type != "c" {
		t.Fatalf("expected challenge 'c', got %q", reply.Type)
	}
	var cp shared.ChallengePacket
	if err := json.Unmarshal(reply.Message, &cp); err != nil {
		t.Fatalf("unmarshal ChallengePacket failed: %v", err)
	}
	if cp.PendingAuth == "" {
		t.Fatal("challenge carried an empty nonce")
	}
	return cp
}

// respondChallenge signs the nonce with priv and sends the challenge response.
func respondChallenge(t *testing.T, conn *websocket.Conn, priv ed25519.PrivateKey, nonce string) {
	t.Helper()
	resp := shared.ChallengeResponse{
		Signature: ed25519.Sign(priv, []byte(nonce)),
		Nonce:     nonce,
	}
	writeGeneralPacket(t, conn, shared.GeneralPacket{Type: "c", Message: mustMarshal(t, resp)})
}

// expectDir reads the next packet and asserts it is a directory packet.
func expectDir(t *testing.T, conn *websocket.Conn) shared.DirPacket {
	t.Helper()
	reply := readGeneralPacket(t, conn)
	if reply.Type != "d" {
		t.Fatalf("expected directory 'd', got %q", reply.Type)
	}
	var dir shared.DirPacket
	if err := json.Unmarshal(reply.Message, &dir); err != nil {
		t.Fatalf("unmarshal DirPacket failed: %v", err)
	}
	return dir
}

// expectErr reads the next packet and asserts it is an error packet.
func expectErr(t *testing.T, conn *websocket.Conn) shared.ErrPacket {
	t.Helper()
	reply := readGeneralPacket(t, conn)
	if reply.Type != "e" {
		t.Fatalf("expected error 'e', got %q", reply.Type)
	}
	var ep shared.ErrPacket
	if err := json.Unmarshal(reply.Message, &ep); err != nil {
		t.Fatalf("unmarshal ErrPacket failed: %v", err)
	}
	return ep
}

// completeChallengeHandshake performs the full handshake + challenge-response
// flow with a fresh identity key: dial, handshake, sign the issued nonce, and
// drain the directory packet plus the join broadcast the server sends back.
func completeChallengeHandshake(t *testing.T, ts *httptest.Server, nickname string, hpke []byte) (*websocket.Conn, ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv := newIdentity(t)
	conn := dialTestServer(t, ts)
	t.Cleanup(func() { conn.Close(websocket.StatusNormalClosure, "") })

	sendHandshake(t, conn, nickname, pub, hpke)
	cp := expectChallenge(t, conn)
	respondChallenge(t, conn, priv, cp.PendingAuth)

	dir := expectDir(t, conn)
	if hpkeGot := dir.CurrentUsers[nickname]; string(hpkeGot) != string(hpke) {
		t.Fatalf("directory must carry the HPKE key for %s, got %q", nickname, hpkeGot)
	}
	// drain our own join broadcast
	if r := readGeneralPacket(t, conn); r.Type != "j" {
		t.Fatalf("%s expected own 'j' broadcast, got %q", nickname, r.Type)
	}
	return conn, pub, priv
}

// TestHandshakeNewUserChallengeFlow proves the full all-users challenge flow
// for a brand new user: handshake -> challenge -> valid signature ->
// directory + join broadcast, with the identity key persisted (TOFU).
func TestHandshakeNewUserChallengeFlow(t *testing.T) {
	hub, ctx, _ := newTestHub(t)
	ts := httptest.NewServer(NewServerHandler(ctx, hub))
	defer ts.Close()

	// A witness client that can observe join broadcasts.
	watcher, _, _ := completeChallengeHandshake(t, ts, "watcher", []byte("hpke-watcher"))

	pub, priv := newIdentity(t)
	conn := dialTestServer(t, ts)
	defer conn.Close(websocket.StatusNormalClosure, "")

	// A new user's handshake must be answered with a challenge, not a
	// directory packet.
	sendHandshake(t, conn, "alice", pub, []byte("hpke-alice"))
	cp := expectChallenge(t, conn)
	if cp.Nickname != "alice" {
		t.Fatalf("challenge names %q, want alice", cp.Nickname)
	}
	if !bytes.Equal(cp.PubKey, pub) {
		t.Fatal("challenge does not echo the presented identity key")
	}

	// A valid signature completes the handshake with a directory packet that
	// carries the HPKE key, not the identity key.
	respondChallenge(t, conn, priv, cp.PendingAuth)
	dir := expectDir(t, conn)
	if hpkeGot := dir.CurrentUsers["alice"]; string(hpkeGot) != "hpke-alice" {
		t.Fatalf("directory must carry the HPKE key for alice, got %q", hpkeGot)
	}

	// The witness observes the server-originated join broadcast.
	jr := readGeneralPacket(t, watcher)
	if jr.Type != "j" {
		t.Fatalf("witness expected alice's 'j', got %q", jr.Type)
	}
	var jp shared.JoinPacket
	if err := json.Unmarshal(jr.Message, &jp); err != nil {
		t.Fatalf("unmarshal JoinPacket failed: %v", err)
	}
	if jp.Nickname != "alice" || string(jp.PubKey) != "hpke-alice" {
		t.Fatalf("unexpected join packet: %+v", jp)
	}

	// The identity key is persisted (TOFU at challenge issuance).
	stored, exists, err := hub.db.GetUserPK("alice")
	if err != nil {
		t.Fatalf("lookup alice failed: %v", err)
	}
	if !exists || !bytes.Equal(stored, pub) {
		t.Fatalf("identity key not stored for alice: exists=%v", exists)
	}

	// The connection is marked verified.
	if c := hub.getClientByNickname("alice"); c == nil || !c.Verified {
		t.Fatal("alice should be verified after a valid challenge response")
	}
}

// TestHandshakeBadSignatureRejected proves a new user presenting a signature
// from the wrong key gets an error, is not marked verified, triggers no join
// broadcast, and never reaches the directory.
func TestHandshakeBadSignatureRejected(t *testing.T) {
	hub, ctx, _ := newTestHub(t)
	ts := httptest.NewServer(NewServerHandler(ctx, hub))
	defer ts.Close()

	watcher, _, _ := completeChallengeHandshake(t, ts, "watcher", []byte("hpke-watcher"))

	pub, _ := newIdentity(t)
	_, otherPriv := newIdentity(t)

	conn := dialTestServer(t, ts)
	defer conn.Close(websocket.StatusNormalClosure, "")

	sendHandshake(t, conn, "mallory", pub, []byte("hpke-mallory"))
	cp := expectChallenge(t, conn)

	// Sign the nonce with the wrong private key.
	resp := shared.ChallengeResponse{
		Signature: ed25519.Sign(otherPriv, []byte(cp.PendingAuth)),
		Nonce:     cp.PendingAuth,
	}
	writeGeneralPacket(t, conn, shared.GeneralPacket{Type: "c", Message: mustMarshal(t, resp)})

	expectErr(t, conn)

	// The connection must not be verified and nothing may be broadcast.
	if c := hub.getClientByNickname("mallory"); c == nil || c.Verified {
		t.Fatal("failed challenge must not mark the connection verified")
	}
	expectNoPacket(t, watcher, "witness")

	// NOTE: the current TOFU flow persists the candidate identity at challenge
	// issuance, before any signature is verified — so the row exists even
	// though authentication failed.
	stored, exists, err := hub.db.GetUserPK("mallory")
	if err != nil {
		t.Fatalf("lookup mallory failed: %v", err)
	}
	if !exists || !bytes.Equal(stored, pub) {
		t.Fatalf("expected TOFU row for mallory at challenge issuance: exists=%v", exists)
	}
}

// TestHandshakeReturningUserChallengeFlow proves a returning user (same
// nickname and identity key) is still challenged — no immediate directory, no
// duplicate rejection — and a valid signature succeeds again.
func TestHandshakeReturningUserChallengeFlow(t *testing.T) {
	hub, ctx, _ := newTestHub(t)
	ts := httptest.NewServer(NewServerHandler(ctx, hub))
	defer ts.Close()

	// First contact: complete the full flow.
	conn1, pub, priv := completeChallengeHandshake(t, ts, "carol", []byte("hpke-carol"))
	defer conn1.Close(websocket.StatusNormalClosure, "")

	// Reconnect with the same identity: still a challenge.
	conn2 := dialTestServer(t, ts)
	defer conn2.Close(websocket.StatusNormalClosure, "")
	sendHandshake(t, conn2, "carol", pub, []byte("hpke-carol"))
	cp := expectChallenge(t, conn2)
	respondChallenge(t, conn2, priv, cp.PendingAuth)

	dir := expectDir(t, conn2)
	if hpkeGot := dir.CurrentUsers["carol"]; string(hpkeGot) != "hpke-carol" {
		t.Fatalf("directory must carry the HPKE key for carol, got %q", hpkeGot)
	}
	if _, exists, err := hub.db.GetUserPK("carol"); err != nil || !exists {
		t.Fatalf("carol should remain stored: exists=%v err=%v", exists, err)
	}
}

// TestHandshakeSameNameDifferentKeyGetsSuffix proves a second identity claiming
// an existing nickname is not rejected outright: it is assigned a suffixed
// nickname and must then complete the challenge flow under that name.
func TestHandshakeSameNameDifferentKeyGetsSuffix(t *testing.T) {
	hub, ctx, _ := newTestHub(t)
	ts := httptest.NewServer(NewServerHandler(ctx, hub))
	defer ts.Close()

	conn1, _, _ := completeChallengeHandshake(t, ts, "bob", []byte("hpke-bob1"))
	defer conn1.Close(websocket.StatusNormalClosure, "")

	pub2, priv2 := newIdentity(t)
	conn2 := dialTestServer(t, ts)
	defer conn2.Close(websocket.StatusNormalClosure, "")

	sendHandshake(t, conn2, "bob", pub2, []byte("hpke-bob2"))

	// The server assigns a suffixed nickname instead of rejecting.
	reply := readGeneralPacket(t, conn2)
	if reply.Type != "n" {
		t.Fatalf("expected nickname update 'n', got %q", reply.Type)
	}
	var nu shared.NicknameUpdatePacket
	if err := json.Unmarshal(reply.Message, &nu); err != nil {
		t.Fatalf("unmarshal NicknameUpdatePacket failed: %v", err)
	}
	if nu.Nickname == "bob" || !strings.HasPrefix(nu.Nickname, "bob-") {
		t.Fatalf("expected a 'bob-' suffixed nickname, got %q", nu.Nickname)
	}
	suffixed := nu.Nickname

	// The suffixed identity must prove itself via the challenge flow. NOTE:
	// the server currently issues one challenge per nonce on this path, so an
	// extra challenge may arrive before the directory; skip any extras.
	cp := expectChallenge(t, conn2)
	respondChallenge(t, conn2, priv2, cp.PendingAuth)
	for {
		r := readGeneralPacket(t, conn2)
		switch r.Type {
		case "c":
			continue // redundant challenge nonce; ignore
		case "d":
			var dir shared.DirPacket
			if err := json.Unmarshal(r.Message, &dir); err != nil {
				t.Fatalf("unmarshal DirPacket failed: %v", err)
			}
			if hpkeGot := dir.CurrentUsers[suffixed]; string(hpkeGot) != "hpke-bob2" {
				t.Fatalf("directory must carry the HPKE key for %s, got %q", suffixed, hpkeGot)
			}
			stored, exists, err := hub.db.GetUserPK(suffixed)
			if err != nil || !exists || !bytes.Equal(stored, pub2) {
				t.Fatalf("suffixed identity not stored: exists=%v err=%v", exists, err)
			}
			return
		default:
			t.Fatalf("unexpected packet %q while waiting for directory", r.Type)
		}
	}
}

func TestHandshakeMalformed(t *testing.T) {
	hub, ctx, _ := newTestHub(t)
	ts := httptest.NewServer(NewServerHandler(ctx, hub))
	defer ts.Close()

	conn := dialTestServer(t, ts)
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Send valid GeneralPacket envelope, but inner payload is a JSON string
	// (not an object) — HandleHandshake's unmarshal into HandshakePacket fails.
	gp := shared.GeneralPacket{Type: "h", Message: json.RawMessage(`"not-an-object"`)}
	writeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := wsjson.Write(writeCtx, conn, gp); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	reply := readGeneralPacket(t, conn)
	if reply.Type != "e" {
		t.Fatalf("expected type 'e', got %q", reply.Type)
	}
}
