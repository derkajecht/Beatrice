package server

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestOversizedFrameRejected(t *testing.T) {
	hub, ctx, _ := newTestHub(t)
	// Long timeout to isolate the read-limit check from inactivity detection
	hub.InactivityTimeout = 90 * time.Second

	ts := httptest.NewServer(NewServerHandler(ctx, hub))
	defer ts.Close()

	url := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	dialCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(dialCtx, url, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Write a 2 MiB payload — exceeds the server's 1 MiB read limit
	writeCtx, wCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer wCancel()
	payload := make([]byte, 2<<20)
	if err := conn.Write(writeCtx, websocket.MessageText, payload); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Server should close with StatusMessageTooBig (1009)
	readCtx, rCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer rCancel()
	_, _, err = conn.Read(readCtx)

	var ce websocket.CloseError
	if !errors.As(err, &ce) {
		t.Fatalf("expected CloseError, got: %v", err)
	}
	if ce.Code != websocket.StatusMessageTooBig {
		t.Fatalf("expected status %d (StatusMessageTooBig), got %d", websocket.StatusMessageTooBig, ce.Code)
	}
}

func TestSilentConnRemoved(t *testing.T) {
	hub, ctx, _ := newTestHub(t)
	hub.InactivityTimeout = 2 * time.Second

	ts := httptest.NewServer(NewServerHandler(ctx, hub))
	defer ts.Close()

	url := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	dialCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(dialCtx, url, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Send nothing — server should drop the connection after InactivityTimeout
	readCtx, rCancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer rCancel()
	_, _, err = conn.Read(readCtx)

	if err == nil {
		t.Fatal("expected error from read on dropped connection, got nil")
	}
}
