package server_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/anomalyco/myserver/daemon/internal/exec"
	"github.com/anomalyco/myserver/daemon/internal/protocol"
	"github.com/anomalyco/myserver/daemon/internal/server"
)

func TestWS_ExecCommandFlow(t *testing.T) {
	s := server.New(server.Config{Addr: "127.0.0.1:0", Version: "vtest"})
	eng := exec.NewEngine()
	defer eng.KillAll()

	s.OnMessage(func(ctx context.Context, c *websocket.Conn, msg []byte) {
		var env protocol.Envelope
		if err := json.Unmarshal(msg, &env); err != nil || env.Action != protocol.ActionExecCommand {
			return
		}
		var payload protocol.ExecRequest
		if err := env.Decode(&payload); err != nil {
			return
		}
		// immediate ack
		ack, _ := protocol.NewResponse(env.ID, protocol.ExecResponse{Status: "ok", Level: 0})
		ackData, _ := json.Marshal(ack)
		wctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		_ = c.Write(wctx, websocket.MessageText, ackData)

		// run + stream
		var mu sync.Mutex
		_, _ = eng.Run(ctx, exec.Options{Command: payload.Command, Timeout: 3 * time.Second}, func(ev exec.StreamEvent) {
			if ev.Stream != "stdout" {
				return
			}
			stream, _ := protocol.NewStream(env.ID, protocol.ExecStreamEvent{
				RequestID: env.ID,
				Stream:    ev.Stream,
				Data:      ev.Data,
				Done:      ev.Done,
			})
			data, _ := json.Marshal(stream)
			mu.Lock()
			defer mu.Unlock()
			wctx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			_ = c.Write(wctx, websocket.MessageText, data)
		})
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := s.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer s.Shutdown(context.Background())

	wctx, wcancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer wcancel()
	c, _, err := websocket.Dial(wctx, "ws://"+s.Addr()+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()

	// consume hello
	_, _, err = c.Read(wctx)
	if err != nil {
		t.Fatal(err)
	}

	req, _ := protocol.NewRequest("exec-1", protocol.ActionExecCommand, protocol.ExecRequest{
		Command: `printf "ok-stream\n"`,
	})
	data, _ := json.Marshal(req)
	if err := c.Write(wctx, websocket.MessageText, data); err != nil {
		t.Fatal(err)
	}

	// ack frame
	_, ackRaw, err := c.Read(wctx)
	if err != nil {
		t.Fatal(err)
	}
	var ack protocol.Envelope
	if err := json.Unmarshal(ackRaw, &ack); err != nil {
		t.Fatal(err)
	}
	if ack.Type != protocol.TypeResponse {
		t.Fatalf("first frame should be response, got %s", ack.Type)
	}

	// stream frame
	_, streamRaw, err := c.Read(wctx)
	if err != nil {
		t.Fatal(err)
	}
	var stream protocol.Envelope
	if err := json.Unmarshal(streamRaw, &stream); err != nil {
		t.Fatal(err)
	}
	if stream.Type != protocol.TypeStream {
		t.Fatalf("expected stream, got %s", stream.Type)
	}
	var se protocol.ExecStreamEvent
	_ = stream.Decode(&se)
	if se.Data != "ok-stream" {
		t.Fatalf("stream data=%q", se.Data)
	}
}
