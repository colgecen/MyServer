package app_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/anomalyco/myserver/daemon/internal/app"
	"github.com/anomalyco/myserver/daemon/internal/protocol"
	"github.com/anomalyco/myserver/daemon/internal/server"
)

func startApp(t *testing.T) (string, func()) {
	t.Helper()
	srv := server.New(server.Config{Addr: "127.0.0.1:0", Version: "test"})
	app.New(srv)
	ctx, cancel := context.WithCancel(context.Background())
	if err := srv.Start(ctx); err != nil {
		cancel()
		t.Fatal(err)
	}
	return "ws://" + srv.Addr() + "/ws", func() {
		cancel()
		_ = srv.Shutdown(context.Background())
	}
}

func dial(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	// consume hello frame
	if _, _, err := c.Read(ctx); err != nil {
		t.Fatal(err)
	}
	return c
}

// waitForReply reads frames until one matches the request id (skipping
// server-pushed events whose ids never collide with request ids).
func waitForReply(t *testing.T, c *websocket.Conn, id string, wait time.Duration) *protocol.Envelope {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	for {
		_, raw, err := c.Read(ctx)
		if err != nil {
			return nil
		}
		var e protocol.Envelope
		if json.Unmarshal(raw, &e) != nil {
			continue
		}
		if e.ID == id {
			return &e
		}
	}
}

func TestIPC_WorkspaceSelectionAndIndex(t *testing.T) {
	url, stop := startApp(t)
	defer stop()
	c := dial(t, url)

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	req, _ := protocol.NewRequest("ipc-1", protocol.ActionWorkspacePick, protocol.WorkspacePickRequest{Path: root})
	data, _ := json.Marshal(req)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := c.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatal(err)
	}
	resp := waitForReply(t, c, "ipc-1", 15*time.Second)
	if resp == nil {
		t.Fatal("no reply for ipc-1")
	}
	if resp.Type == protocol.TypeError {
		t.Fatalf("index error: %s %s", resp.Error.Code, resp.Error.Message)
	}
	var ir protocol.IndexResponse
	if err := resp.Decode(&ir); err != nil {
		t.Fatal(err)
	}
	if ir.Status != "ok" || ir.FileCount < 1 {
		t.Fatalf("unexpected index response: %+v", ir)
	}
}

func TestIPC_WorkspaceRejectsBadPath(t *testing.T) {
	url, stop := startApp(t)
	defer stop()
	c := dial(t, url)

	req, _ := protocol.NewRequest("ipc-2", protocol.ActionWorkspacePick, protocol.WorkspacePickRequest{Path: "/nonexistent-dir-xyz"})
	data2, _ := json.Marshal(req)
	wctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	if err := c.Write(wctx2, websocket.MessageText, data2); err != nil {
		t.Fatal(err)
	}
	resp := waitForReply(t, c, "ipc-2", 5*time.Second)
	if resp == nil || resp.Type != protocol.TypeError {
		t.Fatalf("expected error envelope, got %+v", resp)
	}
	if resp.Error == nil || resp.Error.Code != "E_WORKSPACE" {
		t.Fatalf("wrong error: %+v", resp.Error)
	}
}

func TestIPC_UnknownAction(t *testing.T) {
	url, stop := startApp(t)
	defer stop()
	c := dial(t, url)

	req, _ := protocol.NewRequest("ipc-3", "bogus_action", nil)
	data3, _ := json.Marshal(req)
	wctx3, cancel3 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel3()
	if err := c.Write(wctx3, websocket.MessageText, data3); err != nil {
		t.Fatal(err)
	}
	resp := waitForReply(t, c, "ipc-3", 5*time.Second)
	if resp == nil || resp.Type != protocol.TypeError {
		t.Fatalf("expected error envelope, got %+v", resp)
	}
}