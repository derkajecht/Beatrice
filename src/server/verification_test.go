package server

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/derkajecht/Beatrice/src/shared"
)

func TestChallengeVerificationSuccess(t *testing.T) {
	hub, ctx, _ := newTestHub(t)
	ts := httptest.NewServer(NewServerHandler(ctx, hub))
	defer ts.Close()

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	// Register a new user (trust on first use).
	conn1 := dialTestServer(t, ts)
	defer conn1.Close(websocket.StatusNormalClosure, "")
	hs1 := shared.HandshakePacket{Nickname: "carol", PubKey: pub}
	gp1 := shared.GeneralPacket{Type: "h", Message: mustMarshal(t, hs1)}
	writeCtx1, cancel1 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel1()
	if err := wsjson.Write(writeCtx1, conn1, gp1); err != nil {
		t.Fatalf("client1 write failed: %v", err)
	}
	if reply := readGeneralPacket(t, conn1); reply.Type != "d" {
		t.Fatalf("new user expected 'd', got %q", reply.Type)
	}

	// Reconnect as a returning user — server must issue a challenge.
	conn2 := dialTestServer(t, ts)
	defer conn2.Close(websocket.StatusNormalClosure, "")
	hs2 := shared.HandshakePacket{Nickname: "carol", PubKey: pub}
	gp2 := shared.GeneralPacket{Type: "h", Message: mustMarshal(t, hs2)}
	writeCtx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	if err := wsjson.Write(writeCtx2, conn2, gp2); err != nil {
		t.Fatalf("client2 write failed: %v", err)
	}
	reply2 := readGeneralPacket(t, conn2)
	if reply2.Type != "c" {
		t.Fatalf("returning user expected 'c', got %q", reply2.Type)
	}
	var cp shared.ChallengePacket
	if err := json.Unmarshal(reply2.Message, &cp); err != nil {
		t.Fatalf("unmarshal ChallengePacket failed: %v", err)
	}

	// Sign the nonce and respond.
	sig := ed25519.Sign(priv, []byte(cp.PendingAuth))
	resp := shared.ChallengeResponse{Signature: sig, Nonce: cp.PendingAuth}
	gp3 := shared.GeneralPacket{Type: "c", Message: mustMarshal(t, resp)}
	writeCtx3, cancel3 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel3()
	if err := wsjson.Write(writeCtx3, conn2, gp3); err != nil {
		t.Fatalf("challenge response write failed: %v", err)
	}
	if reply3 := readGeneralPacket(t, conn2); reply3.Type != "d" {
		t.Fatalf("verified user expected 'd', got %q", reply3.Type)
	}
}

func TestChallengeRejectsBadSignature(t *testing.T) {
	hub, ctx, _ := newTestHub(t)
	ts := httptest.NewServer(NewServerHandler(ctx, hub))
	defer ts.Close()

	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	_, otherPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate other key: %v", err)
	}

	// Register.
	conn1 := dialTestServer(t, ts)
	defer conn1.Close(websocket.StatusNormalClosure, "")
	hs1 := shared.HandshakePacket{Nickname: "dave", PubKey: pub}
	gp1 := shared.GeneralPacket{Type: "h", Message: mustMarshal(t, hs1)}
	writeCtx1, cancel1 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel1()
	if err := wsjson.Write(writeCtx1, conn1, gp1); err != nil {
		t.Fatalf("client1 write failed: %v", err)
	}
	if reply := readGeneralPacket(t, conn1); reply.Type != "d" {
		t.Fatalf("new user expected 'd', got %q", reply.Type)
	}

	// Reconnect, get challenge, respond with a signature from the wrong key.
	conn2 := dialTestServer(t, ts)
	defer conn2.Close(websocket.StatusNormalClosure, "")
	hs2 := shared.HandshakePacket{Nickname: "dave", PubKey: pub}
	gp2 := shared.GeneralPacket{Type: "h", Message: mustMarshal(t, hs2)}
	writeCtx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	if err := wsjson.Write(writeCtx2, conn2, gp2); err != nil {
		t.Fatalf("client2 write failed: %v", err)
	}
	reply2 := readGeneralPacket(t, conn2)
	if reply2.Type != "c" {
		t.Fatalf("returning user expected 'c', got %q", reply2.Type)
	}
	var cp shared.ChallengePacket
	if err := json.Unmarshal(reply2.Message, &cp); err != nil {
		t.Fatalf("unmarshal ChallengePacket failed: %v", err)
	}

	badSig := ed25519.Sign(otherPriv, []byte(cp.PendingAuth))
	resp := shared.ChallengeResponse{Signature: badSig, Nonce: cp.PendingAuth}
	gp3 := shared.GeneralPacket{Type: "c", Message: mustMarshal(t, resp)}
	writeCtx3, cancel3 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel3()
	if err := wsjson.Write(writeCtx3, conn2, gp3); err != nil {
		t.Fatalf("challenge response write failed: %v", err)
	}
	if reply3 := readGeneralPacket(t, conn2); reply3.Type != "e" {
		t.Fatalf("bad signature expected 'e', got %q", reply3.Type)
	}
}
