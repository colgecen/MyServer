package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/anomalyco/myserver/daemon/internal/protocol"
	"github.com/anomalyco/myserver/daemon/internal/server"
)

// startTestServer spins up a daemon server on a free loopback port and returns
// its base URL plus a shutdown func.
func startTestServer(t *testing.T) (*server.HTTPServer, string, func()) {
	t.Helper()
	s := server.New(server.Config{Addr: "127.0.0.1:0", Version: "test"})
	// echo handler replies with the same id and a response envelope.
	s.OnMessage(func(ctx context.Context, c *websocket.Conn, msg []byte) {
		var env protocol.Envelope
		if err := json.Unmarshal(msg, &env); err != nil {
			return
		}
		resp, _ := protocol.NewResponse(env.ID, map[string]string{"ok": "1"})
		_ = resp
		data, _ := json.Marshal(resp)
		wctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		_ = c.Write(wctx, websocket.MessageText, data)
	})
	ctx, cancel := context.WithCancel(context.Background())
	if err := s.Start(ctx); err != nil {
		cancel()
		t.Fatalf("start: %v", err)
	}
	return s, "ws://" + s.Addr() + "/ws", func() {
		cancel()
		_ = s.Shutdown(context.Background())
	}
}

func TestHealthEndpoint(t *testing.T) {
	s := server.New(server.Config{Addr: "127.0.0.1:0", Version: "vtest"})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := s.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer s.Shutdown(context.Background())

	resp, err := http.Get("http://" + s.Addr() + "/api/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" || body["version"] != "vtest" {
		t.Fatalf("body=%v", body)
	}
}

func TestWebSocketEcho(t *testing.T) {
	s, wsURL, stop := startTestServer(t)
	defer stop()
	_ = s

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()

	// first frame is the server hello
	_, helloData, err := c.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var hello protocol.Envelope
	if err := json.Unmarshal(helloData, &hello); err != nil {
		t.Fatal(err)
	}
	if hello.Type != protocol.TypeEvent || hello.Action != "" {
		// accept either: we emit a "hello" event from the server
		if !strings.Contains(string(helloData), `"hello"`) {
			t.Fatalf("unexpected hello frame: %s", helloData)
		}
	}

	req, _ := protocol.NewRequest("req-1", protocol.ActionHealth, nil)
	data, _ := json.Marshal(req)
	if err := c.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatal(err)
	}
	_, respData, err := c.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var resp protocol.Envelope
	if err := json.Unmarshal(respData, &resp); err != nil {
		t.Fatal(err)
	}
	if resp.ID != "req-1" {
		t.Fatalf("resp id=%s", resp.ID)
	}
}
