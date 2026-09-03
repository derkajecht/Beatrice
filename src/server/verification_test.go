package server

import (
	"crypto/ed25519"
	"net/http/httptest"
	"testing"

	"github.com/coder/websocket"
	"github.com/derkajecht/Beatrice/src/shared"
)

// TestChallengeVerificationSuccess proves a new user must answer a challenge
// before receiving the directory, and a returning user gets a fresh challenge
// that a valid signature satisfies again.
func TestChallengeVerificationSuccess(t *testing.T) {
	hub, ctx, _ := newTestHub(t)
	ts := httptest.NewServer(NewServerHandler(ctx, hub))
	defer ts.Close()

	pub, priv := newIdentity(t)

	// New user: the handshake is answered with a challenge, not a directory.
	conn1 := dialTestServer(t, ts)
	defer conn1.Close(websocket.StatusNormalClosure, "")
	sendHandshake(t, conn1, "carol", pub, []byte("hpke-carol"))
	cp := expectChallenge(t, conn1)
	respondChallenge(t, conn1, priv, cp.PendingAuth)
	expectDir(t, conn1)
	if c := hub.getClientByNickname("carol"); c == nil || !c.Verified {
		t.Fatal("carol should be verified after the challenge response")
	}

	// Returning user: challenged again, valid signature succeeds.
	conn2 := dialTestServer(t, ts)
	defer conn2.Close(websocket.StatusNormalClosure, "")
	sendHandshake(t, conn2, "carol", pub, []byte("hpke-carol"))
	cp = expectChallenge(t, conn2)
	respondChallenge(t, conn2, priv, cp.PendingAuth)
	dir := expectDir(t, conn2)
	if hpkeGot := dir.CurrentUsers["carol"]; string(hpkeGot) != "hpke-carol" {
		t.Fatalf("directory must carry the HPKE key for carol, got %q", hpkeGot)
	}
}

// TestChallengeRejectsBadSignature proves a signature from the wrong key
// cannot authenticate: the client gets an error and stays unverified.
func TestChallengeRejectsBadSignature(t *testing.T) {
	hub, ctx, _ := newTestHub(t)
	ts := httptest.NewServer(NewServerHandler(ctx, hub))
	defer ts.Close()

	pub, _ := newIdentity(t)
	_, otherPriv := newIdentity(t)

	conn := dialTestServer(t, ts)
	defer conn.Close(websocket.StatusNormalClosure, "")

	// New user is challenged; a signature from the wrong key fails.
	sendHandshake(t, conn, "dave", pub, []byte("hpke-dave"))
	cp := expectChallenge(t, conn)
	resp := shared.ChallengeResponse{
		Signature: ed25519.Sign(otherPriv, []byte(cp.PendingAuth)),
		Nonce:     cp.PendingAuth,
	}
	writeGeneralPacket(t, conn, shared.GeneralPacket{Type: "c", Message: mustMarshal(t, resp)})

	if ep := expectErr(t, conn); ep.Message != "err_challenge_failed" {
		t.Fatalf("unexpected error message: %q", ep.Message)
	}
	if c := hub.getClientByNickname("dave"); c == nil || c.Verified {
		t.Fatal("failed challenge must not mark the connection verified")
	}
}

// TestChallengeNonceReplayRejected proves a challenge nonce is burned on use:
// replaying a successful challenge response cannot authenticate again.
func TestChallengeNonceReplayRejected(t *testing.T) {
	hub, ctx, _ := newTestHub(t)
	ts := httptest.NewServer(NewServerHandler(ctx, hub))
	defer ts.Close()

	// First contact: complete the full flow.
	conn1, pub, priv := completeChallengeHandshake(t, ts, "erin", []byte("hpke-erin"))
	defer conn1.Close(websocket.StatusNormalClosure, "")

	// Reconnect and complete the challenge legitimately.
	conn2 := dialTestServer(t, ts)
	defer conn2.Close(websocket.StatusNormalClosure, "")
	sendHandshake(t, conn2, "erin", pub, []byte("hpke-erin"))
	cp := expectChallenge(t, conn2)
	resp := shared.ChallengeResponse{
		Signature: ed25519.Sign(priv, []byte(cp.PendingAuth)),
		Nonce:     cp.PendingAuth,
	}
	writeGeneralPacket(t, conn2, shared.GeneralPacket{Type: "c", Message: mustMarshal(t, resp)})
	expectDir(t, conn2)
	if r := readGeneralPacket(t, conn2); r.Type != "j" {
		t.Fatalf("expected own 'j' broadcast, got %q", r.Type)
	}

	// Replaying the exact same response must fail: the nonce is consumed.
	writeGeneralPacket(t, conn2, shared.GeneralPacket{Type: "c", Message: mustMarshal(t, resp)})
	expectErr(t, conn2)
}
