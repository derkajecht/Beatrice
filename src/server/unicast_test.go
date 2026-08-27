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

// handshakeClient dials the test server and completes a TOFU handshake,
// draining the directory packet and join broadcasts that follow. The
// handshake carries both the identity key and the HPKE public key, exactly
// like the real client.
func handshakeClient(t *testing.T, ts *httptest.Server, nickname string) *websocket.Conn {
	t.Helper()
	conn := dialTestServer(t, ts)
	t.Cleanup(func() { conn.Close(websocket.StatusNormalClosure, "") })

	hs := shared.HandshakePacket{
		Nickname:   nickname,
		PubKey:     []byte("identity-" + nickname),
		HPKEPubKey: []byte("hpke-" + nickname),
	}
	gp := shared.GeneralPacket{Type: "h", Message: mustMarshal(t, hs)}
	writeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := wsjson.Write(writeCtx, conn, gp); err != nil {
		t.Fatalf("%s handshake write failed: %v", nickname, err)
	}

	// own directory packet + own join broadcast
	dirReply := readGeneralPacket(t, conn)
	if dirReply.Type != "d" {
		t.Fatalf("%s expected 'd', got %q", nickname, dirReply.Type)
	}
	var dir shared.DirPacket
	if err := json.Unmarshal(dirReply.Message, &dir); err != nil {
		t.Fatalf("unmarshal DirPacket failed: %v", err)
	}
	if string(dir.CurrentUsers[nickname]) != "hpke-"+nickname {
		t.Fatalf("directory must carry the HPKE key for %s, got %q", nickname, dir.CurrentUsers[nickname])
	}
	if r := readGeneralPacket(t, conn); r.Type != "j" {
		t.Fatalf("%s expected 'j', got %q", nickname, r.Type)
	}
	return conn
}

func sendMessage(t *testing.T, conn *websocket.Conn, mp shared.MessagePacket) {
	t.Helper()
	gp := shared.GeneralPacket{Type: "m", Message: mustMarshal(t, mp)}
	writeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := wsjson.Write(writeCtx, conn, gp); err != nil {
		t.Fatalf("message write failed: %v", err)
	}
}

// expectNoPacket asserts nothing arrives within a bounded window.
func expectNoPacket(t *testing.T, conn *websocket.Conn, who string) {
	t.Helper()
	readCtx, cancel := context.WithTimeout(context.Background(), 750*time.Millisecond)
	defer cancel()
	if _, _, err := conn.Read(readCtx); err == nil {
		t.Fatalf("%s received a packet that should not have been sent to it", who)
	}
}

func TestMessageUnicastDeliveredToRecipientOnly(t *testing.T) {
	hub, ctx, _ := newTestHub(t)
	ts := httptest.NewServer(NewServerHandler(ctx, hub))
	defer ts.Close()

	alice := handshakeClient(t, ts, "alice")
	bob := handshakeClient(t, ts, "bob")

	// alice receives bob's join broadcast before any traffic
	if r := readGeneralPacket(t, alice); r.Type != "j" {
		t.Fatalf("alice expected bob's 'j', got %q", r.Type)
	}

	sendMessage(t, alice, shared.MessagePacket{
		Recipient: "bob",
		Sender:    "alice",
		Enc:       []byte("enc-bytes"),
		CT:        []byte("ct-bytes"),
	})

	reply := readGeneralPacket(t, bob)
	if reply.Type != "m" {
		t.Fatalf("bob expected 'm', got %q", reply.Type)
	}
	var mp shared.MessagePacket
	if err := json.Unmarshal(reply.Message, &mp); err != nil {
		t.Fatalf("unmarshal MessagePacket failed: %v", err)
	}
	// sender is bound to the authenticated connection, not the wire field
	if mp.Sender != "alice" || mp.Recipient != "bob" {
		t.Fatalf("unexpected routing: sender=%q recipient=%q", mp.Sender, mp.Recipient)
	}
	if string(mp.Enc) != "enc-bytes" || string(mp.CT) != "ct-bytes" {
		t.Fatalf("payload corrupted in transit: enc=%q ct=%q", mp.Enc, mp.CT)
	}

	// the sender (and only the recipient) must NOT receive its own message
	expectNoPacket(t, alice, "alice (sender)")
}

// TestUnverifiedSenderRejected proves routing requires authentication: a
// connection that never completed the handshake cannot send messages.
func TestUnverifiedSenderRejected(t *testing.T) {
	hub, ctx, _ := newTestHub(t)
	ts := httptest.NewServer(NewServerHandler(ctx, hub))
	defer ts.Close()

	mallory := dialTestServer(t, ts)
	defer mallory.Close(websocket.StatusNormalClosure, "")

	sendMessage(t, mallory, shared.MessagePacket{Sender: "mallory", Recipient: "bob"})
	reply := readGeneralPacket(t, mallory)
	if reply.Type != "e" {
		t.Fatalf("expected 'e' for unverified sender, got %q", reply.Type)
	}
	var ep shared.ErrPacket
	if err := json.Unmarshal(reply.Message, &ep); err != nil {
		t.Fatalf("unmarshal ErrPacket failed: %v", err)
	}
	if ep.Message != "err_unverified_sender" {
		t.Fatalf("unexpected error message: %q", ep.Message)
	}
}

func TestMessageUnknownRecipientGetsErrPacket(t *testing.T) {
	hub, ctx, _ := newTestHub(t)
	ts := httptest.NewServer(NewServerHandler(ctx, hub))
	defer ts.Close()

	alice := handshakeClient(t, ts, "alice")

	// unknown recipient -> error back to the sender, nothing broadcast
	sendMessage(t, alice, shared.MessagePacket{
		Recipient: "carol",
		Sender:    "alice",
		Enc:       []byte("enc"),
		CT:        []byte("ct"),
	})
	reply := readGeneralPacket(t, alice)
	if reply.Type != "e" {
		t.Fatalf("expected 'e' for unknown recipient, got %q", reply.Type)
	}
	var ep shared.ErrPacket
	if err := json.Unmarshal(reply.Message, &ep); err != nil {
		t.Fatalf("unmarshal ErrPacket failed: %v", err)
	}
	if ep.Message != "err_unknown_recipient" {
		t.Fatalf("unexpected error message: %q", ep.Message)
	}

	// missing recipient -> error back to the sender as well
	sendMessage(t, alice, shared.MessagePacket{Sender: "alice", Enc: []byte("e"), CT: []byte("c")})
	reply = readGeneralPacket(t, alice)
	if reply.Type != "e" {
		t.Fatalf("expected 'e' for missing recipient, got %q", reply.Type)
	}
}

// TestClientDirectoryAnnouncementsRejected proves clients cannot poison the
// HPKE directory with forged join/leave announcements.
func TestClientDirectoryAnnouncementsRejected(t *testing.T) {
	hub, ctx, _ := newTestHub(t)
	ts := httptest.NewServer(NewServerHandler(ctx, hub))
	defer ts.Close()

	alice := handshakeClient(t, ts, "alice")

	forge := func(pktType string, pkt any) {
		t.Helper()
		gp := shared.GeneralPacket{Type: pktType, Message: mustMarshal(t, pkt)}
		writeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := wsjson.Write(writeCtx, alice, gp); err != nil {
			t.Fatalf("%s write failed: %v", pktType, err)
		}
		reply := readGeneralPacket(t, alice)
		if reply.Type != "e" {
			t.Fatalf("forged %s: expected 'e' rejection, got %q", pktType, reply.Type)
		}
	}

	forge("j", shared.JoinPacket{Nickname: "mallory", PubKey: []byte("evil")})
	forge("l", shared.LeavePacket{Nickname: "alice"})
}

func TestGetClientByNickname(t *testing.T) {
	hub := NewHub()
	hub.addClient(&ServerClient{ID: "1", Nickname: "alice"})
	hub.addClient(&ServerClient{ID: "2", Nickname: "bob"})

	if c := hub.getClientByNickname("bob"); c == nil || c.ID != "2" {
		t.Fatalf("expected client 2 for bob, got %+v", c)
	}
	if c := hub.getClientByNickname("carol"); c != nil {
		t.Fatalf("expected nil for unknown nickname, got %+v", c)
	}
}
