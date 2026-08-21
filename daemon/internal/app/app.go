// Package app composes the daemon's subsystems and dispatches WebSocket
// protocol actions to them. This is the IPC layer between GUI and backend.
package app

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/anomalyco/myserver/daemon/internal/exec"
	"github.com/anomalyco/myserver/daemon/internal/indexer"
	"github.com/anomalyco/myserver/daemon/internal/protocol"
	"github.com/anomalyco/myserver/daemon/internal/server"
	"github.com/anomalyco/myserver/daemon/internal/workspace"
)

// App owns all subsystems and routes protocol envelopes.
type App struct {
	srv      *server.HTTPServer
	execEng  *exec.Engine
	indexSvc *indexer.Service

	mu    sync.Mutex
	ws    *workspace.Workspace
}

// New wires everything together.
func New(srv *server.HTTPServer) *App {
	a := &App{
		srv:     srv,
		execEng: exec.NewEngine(),
		indexSvc: indexer.NewService(
			indexer.NewOllamaEmbedder("", ""),
			func(p indexer.Progress) {
				ev, err := protocol.NewEvent(protocol.ActionIndexWorkspace, p)
				if err == nil {
					srv.Broadcast(ev)
				}
			},
		),
	}
	srv.OnMessage(a.handleMessage)
	return a
}

// handleMessage is the single inbound WS entry point.
func (a *App) handleMessage(ctx context.Context, c *websocket.Conn, raw []byte) {
	var env protocol.Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		a.reply(ctx, c, protocol.NewError("unknown", "E_BADJSON", err.Error()))
		return
	}
	if err := env.Validate(); err != nil {
		a.reply(ctx, c, protocol.NewError(env.ID, "E_BADREQ", err.Error()))
		return
	}

	switch env.Action {
	case protocol.ActionHealth:
		a.reply(ctx, c, must(protocol.NewResponse(env.ID, map[string]string{"status": "ok"})))

	case protocol.ActionWorkspacePick:
		var req protocol.WorkspacePickRequest
		if err := env.Decode(&req); err != nil {
			a.reply(ctx, c, protocol.NewError(env.ID, "E_PAYLOAD", err.Error()))
			return
		}
		a.selectWorkspace(ctx, c, env.ID, req.Path)

	case protocol.ActionIndexWorkspace:
		var req protocol.IndexRequest
		if err := env.Decode(&req); err != nil {
			a.reply(ctx, c, protocol.NewError(env.ID, "E_PAYLOAD", err.Error()))
			return
		}
		go a.indexWorkspace(c, env.ID, req.Path)

	case protocol.ActionExecCommand:
		var req protocol.ExecRequest
		if err := env.Decode(&req); err != nil {
			a.reply(ctx, c, protocol.NewError(env.ID, "E_PAYLOAD", err.Error()))
			return
		}
		go a.execCommand(ctx, c, &env, req)

	default:
		a.reply(ctx, c, protocol.NewError(env.ID, "E_ACTION", "unsupported action: "+string(env.Action)))
	}
}

// selectWorkspace validates the path against the sandbox then re-indexes.
func (a *App) selectWorkspace(ctx context.Context, c *websocket.Conn, id, path string) {
	ws, err := workspace.New(path)
	if err != nil {
		a.reply(ctx, c, protocol.NewError(id, "E_WORKSPACE", err.Error()))
		return
	}
	a.mu.Lock()
	a.ws = ws
	a.mu.Unlock()

	summary, err := a.indexSvc.SelectWorkspace(ctx, ws.Root())
	if err != nil {
		a.reply(ctx, c, protocol.NewError(id, "E_INDEX", err.Error()))
		return
	}
	a.reply(ctx, c, must(protocol.NewResponse(id, protocol.IndexResponse{
		Status:     "ok",
		FileCount:  summary.FileCount,
		ChunkCount: summary.ChunkCount,
		Duration:   summary.Duration.String(),
	})))
}

// indexWorkspace re-indexes an explicit path (no sandbox switch).
func (a *App) indexWorkspace(c *websocket.Conn, id, path string) {
	summary, err := a.indexSvc.SelectWorkspace(context.Background(), path)
	if err != nil {
		a.reply(context.Background(), c, protocol.NewError(id, "E_INDEX", err.Error()))
		return
	}
	a.reply(context.Background(), c, must(protocol.NewResponse(id, protocol.IndexResponse{
		Status:     "ok",
		FileCount:  summary.FileCount,
		ChunkCount: summary.ChunkCount,
		Duration:   summary.Duration.String(),
	})))
}

// execCommand runs a command inside the sandboxed workdir and streams output.
func (a *App) execCommand(ctx context.Context, c *websocket.Conn, env *protocol.Envelope, req protocol.ExecRequest) {
	// resolve workdir through the workspace sandbox when one is active
	workdir := req.Workdir
	if workdir != "" && a.currentWS() != nil {
		resolved, err := a.currentWS().Resolve(workdir)
		if err != nil {
			a.reply(ctx, c, protocol.NewError(env.ID, "E_SANDBOX", err.Error()))
			return
		}
		workdir = resolved
	}

	a.reply(ctx, c, must(protocol.NewResponse(env.ID, protocol.ExecResponse{Status: "ok", Level: 0})))

	_, _ = a.execEng.Run(ctx, exec.Options{
		Command: req.Command,
		Shell:   req.Shell,
		Workdir: workdir,
	}, func(ev exec.StreamEvent) {
		stream, err := protocol.NewStream(env.ID, protocol.ExecStreamEvent{
			RequestID: env.ID,
			Stream:    ev.Stream,
			Data:      ev.Data,
			ExitCode:  iPtr(ev.ExitCode),
			Done:      ev.Done,
			Err:       errText(ev.Err),
		})
		if err != nil {
			return
		}
		a.reply(ctx, c, stream)
	})
}

func (a *App) currentWS() *workspace.Workspace {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.ws
}

func (a *App) reply(ctx context.Context, c *websocket.Conn, env *protocol.Envelope) {
	data, err := json.Marshal(env)
	if err != nil {
		log.Printf("app: marshal reply: %v", err)
		return
	}
	wctx, cancel := context.WithTimeout(ctx, replyWriteTimeout)
	defer cancel()
	_ = c.Write(wctx, websocket.MessageText, data)
}

const replyWriteTimeout = 10 * time.Second

// Shutdown releases background resources.
func (a *App) Shutdown() {
	a.execEng.KillAll()
	a.indexSvc.Shutdown()
}

func iPtr(i int) *int { return &i }

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func must(env *protocol.Envelope, _ error) *protocol.Envelope { return env }