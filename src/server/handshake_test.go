package server

import (
	"context"
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

func TestHandshakeSuccess(t *testing.T) {
	hub, ctx, _ := newTestHub(t)
	ts := httptest.NewServer(NewServerHandler(ctx, hub))
	defer ts.Close()

	conn := dialTestServer(t, ts)
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Send handshake: identity key for auth plus HPKE key for the directory
	hs := shared.HandshakePacket{
		Nickname:   "alice",
		PubKey:     []byte("pk"),
		HPKEPubKey: []byte("hpke-pk"),
	}
	gp := shared.GeneralPacket{Type: "h", Message: mustMarshal(t, hs)}
	writeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := wsjson.Write(writeCtx, conn, gp); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Read directory response
	reply := readGeneralPacket(t, conn)
	if reply.Type != "d" {
		t.Fatalf("expected type 'd', got %q", reply.Type)
	}

	var dir shared.DirPacket
	if err := json.Unmarshal(reply.Message, &dir); err != nil {
		t.Fatalf("unmarshal DirPacket failed: %v", err)
	}
	// The directory must carry the HPKE key, not the identity key.
	if pk, ok := dir.CurrentUsers["alice"]; !ok {
		t.Fatal("expected 'alice' in CurrentUsers")
	} else if string(pk) != "hpke-pk" {
		t.Fatalf("expected HPKE pubkey 'hpke-pk', got %q", string(pk))
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

func TestHandshakeDuplicateNickname(t *testing.T) {
	hub, ctx, _ := newTestHub(t)
	ts := httptest.NewServer(NewServerHandler(ctx, hub))
	defer ts.Close()

	// Client 1 — successful handshake
	conn1 := dialTestServer(t, ts)
	defer conn1.Close(websocket.StatusNormalClosure, "")
	hs1 := shared.HandshakePacket{Nickname: "bob", PubKey: []byte("pk1")}
	gp1 := shared.GeneralPacket{Type: "h", Message: mustMarshal(t, hs1)}
	writeCtx1, cancel1 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel1()
	if err := wsjson.Write(writeCtx1, conn1, gp1); err != nil {
		t.Fatalf("client1 write failed: %v", err)
	}
	reply1 := readGeneralPacket(t, conn1)
	if reply1.Type != "d" {
		t.Fatalf("client1 expected 'd', got %q", reply1.Type)
	}

	// Client 2 — same nickname, should fail
	conn2 := dialTestServer(t, ts)
	defer conn2.Close(websocket.StatusNormalClosure, "")
	hs2 := shared.HandshakePacket{Nickname: "bob", PubKey: []byte("pk2")}
	gp2 := shared.GeneralPacket{Type: "h", Message: mustMarshal(t, hs2)}
	writeCtx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	if err := wsjson.Write(writeCtx2, conn2, gp2); err != nil {
		t.Fatalf("client2 write failed: %v", err)
	}
	reply2 := readGeneralPacket(t, conn2)
	if reply2.Type != "e" {
		t.Fatalf("client2 expected 'e', got %q", reply2.Type)
	}
}
