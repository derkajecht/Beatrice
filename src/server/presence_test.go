package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/derkajecht/Beatrice/src/shared"
)

// TestPresenceBroadcastBindsAuthenticatedNickname proves presence updates
// are fanned out to all clients with the nickname bound to the authenticated
// connection — a forged wire nickname must be overwritten.
func TestPresenceBroadcastBindsAuthenticatedNickname(t *testing.T) {
	hub, ctx, _ := newTestHub(t)
	ts := httptest.NewServer(NewServerHandler(ctx, hub))
	defer ts.Close()

	alice := handshakeClient(t, ts, "alice")
	bob := handshakeClient(t, ts, "bob")

	// alice receives bob's join broadcast before any traffic
	if r := readGeneralPacket(t, alice); r.Type != "j" {
		t.Fatalf("alice expected bob's 'j', got %q", r.Type)
	}

	// alice sends a presence update with a forged wire nickname
	gp := shared.GeneralPacket{Type: "p", Message: mustMarshal(t, shared.PresencePacket{
		Nickname: "mallory",
		Status:   shared.PresenceAway,
	})}
	writeGeneralPacket(t, alice, gp)

	for who, conn := range map[string]*websocket.Conn{"alice": alice, "bob": bob} {
		reply := readGeneralPacket(t, conn)
		if reply.Type != "p" {
			t.Fatalf("%s expected 'p', got %q", who, reply.Type)
		}
		var pp shared.PresencePacket
		if err := json.Unmarshal(reply.Message, &pp); err != nil {
			t.Fatalf("%s unmarshal PresencePacket failed: %v", who, err)
		}
		if pp.Nickname != "alice" {
			t.Fatalf("%s: nickname must be bound to the authenticated connection, got %q", who, pp.Nickname)
		}
		if pp.Status != shared.PresenceAway {
			t.Fatalf("%s: unexpected status %q", who, pp.Status)
		}
	}
}

// TestPresenceUnverifiedRejected proves presence requires authentication.
func TestPresenceUnverifiedRejected(t *testing.T) {
	hub, ctx, _ := newTestHub(t)
	ts := httptest.NewServer(NewServerHandler(ctx, hub))
	defer ts.Close()

	mallory := dialTestServer(t, ts)
	defer mallory.Close(websocket.StatusNormalClosure, "")

	gp := shared.GeneralPacket{Type: "p", Message: mustMarshal(t, shared.PresencePacket{
		Nickname: "mallory",
		Status:   shared.PresenceActive,
	})}
	writeGeneralPacket(t, mallory, gp)

	reply := readGeneralPacket(t, mallory)
	if reply.Type != "e" {
		t.Fatalf("expected 'e' for unverified presence, got %q", reply.Type)
	}
	var ep shared.ErrPacket
	if err := json.Unmarshal(reply.Message, &ep); err != nil {
		t.Fatalf("unmarshal ErrPacket failed: %v", err)
	}
	if ep.Message != "err_unverified_sender" {
		t.Fatalf("unexpected error message: %q", ep.Message)
	}
}

// TestPresenceInvalidStatusRejected proves malformed statuses are rejected
// before fan-out.
func TestPresenceInvalidStatusRejected(t *testing.T) {
	hub, ctx, _ := newTestHub(t)
	ts := httptest.NewServer(NewServerHandler(ctx, hub))
	defer ts.Close()

	alice := handshakeClient(t, ts, "alice")

	gp := shared.GeneralPacket{Type: "p", Message: mustMarshal(t, shared.PresencePacket{
		Nickname: "alice",
		Status:   "hacking",
	})}
	writeGeneralPacket(t, alice, gp)

	reply := readGeneralPacket(t, alice)
	if reply.Type != "e" {
		t.Fatalf("expected 'e' for invalid status, got %q", reply.Type)
	}
	var ep shared.ErrPacket
	if err := json.Unmarshal(reply.Message, &ep); err != nil {
		t.Fatalf("unmarshal ErrPacket failed: %v", err)
	}
	if ep.Message != "err_invalid_presence_status" {
		t.Fatalf("unexpected error message: %q", ep.Message)
	}
}

// writeGeneralPacket writes a GeneralPacket envelope to the connection.
func writeGeneralPacket(t *testing.T, conn *websocket.Conn, gp shared.GeneralPacket) {
	t.Helper()
	writeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := wsjson.Write(writeCtx, conn, gp); err != nil {
		t.Fatalf("write failed: %v", err)
	}
}
