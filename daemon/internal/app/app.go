// Package app composes the daemon's subsystems and dispatches WebSocket
// protocol actions to them. This is the IPC layer between GUI and backend.
package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/anomalyco/myserver/daemon/internal/exec"
	"github.com/anomalyco/myserver/daemon/internal/guardrail"
	"github.com/anomalyco/myserver/daemon/internal/indexer"
	"github.com/anomalyco/myserver/daemon/internal/llm"
	"github.com/anomalyco/myserver/daemon/internal/protocol"
	"github.com/anomalyco/myserver/daemon/internal/server"
	"github.com/anomalyco/myserver/daemon/internal/store"
	"github.com/anomalyco/myserver/daemon/internal/workspace"
)

// App owns all subsystems and routes protocol envelopes.
type App struct {
	srv      *server.HTTPServer
	execEng  *exec.Engine
	safeExec *guardrail.SafeExec
	indexSvc *indexer.Service
	llmClnt  *llm.Client
	store    *store.KV
	ws       *workspace.Workspace

	mu   sync.Mutex
	user string
}

// New wires everything together.
func New(srv *server.HTTPServer, audit *store.KV) *App {
	safeExec := guardrail.NewWithRateLimit(guardrail.DefaultRateLimit())

	a := &App{
		srv:      srv,
		execEng:  exec.NewEngine(),
		safeExec: safeExec,
		indexSvc: indexer.NewService(
			indexer.NewOllamaEmbedder("", ""),
			func(p indexer.Progress) {
				ev, err := protocol.NewEvent(protocol.ActionIndexWorkspace, p)
				if err == nil {
					srv.Broadcast(ev)
				}
			},
		),
		llmClnt: llm.New(""),
		store:   audit,
		user:    os.Getenv("USER"),
	}
	if a.user == "" {
		a.user = "unknown"
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

	case protocol.ActionExecApprove:
		var req protocol.ExecApprove
		if err := env.Decode(&req); err != nil {
			a.reply(ctx, c, protocol.NewError(env.ID, "E_PAYLOAD", err.Error()))
			return
		}
		a.approveExec(ctx, c, env.ID, req)

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

// approveExec handles user confirmation for Level 1/2 commands.
func (a *App) approveExec(ctx context.Context, c *websocket.Conn, id string, req protocol.ExecApprove) {
	storedReq, class, err := a.safeExec.Approve(req.RequestID, req.Approve)
	if err != nil {
		a.reply(ctx, c, must(protocol.NewResponse(id, protocol.ExecResponse{
			Status:  "error",
			Message: err.Error(),
		})))
		return
	}
	if storedReq == nil {
		a.reply(ctx, c, must(protocol.NewResponse(id, protocol.ExecResponse{
			Status:  "error",
			Message: "request not found or expired",
		})))
		return
	}
	if !req.Approve {
		a.reply(ctx, c, must(protocol.NewResponse(id, protocol.ExecResponse{
			Status:  "denied",
			Message: "command denied by user",
		})))
		return
	}

	// Execute the approved command
	// Send immediate response so client knows approval was accepted
	a.reply(ctx, c, must(protocol.NewResponse(id, protocol.ExecResponse{
		Status:  "approved",
		Message: "command approved, executing...",
		Level:   int(class.Level),
	})))

	workdir := storedReq.Workdir
	if workdir != "" && a.currentWS() != nil {
		resolved, err := a.currentWS().Resolve(workdir)
		if err != nil {
			a.reply(ctx, c, must(protocol.NewResponse(id, protocol.ExecResponse{
				Status:  "error",
				Message: err.Error(),
			})))
			return
		}
		workdir = resolved
	}

	go func() {
		origID := storedReq.ID
		_, _ = a.execEng.Run(ctx, exec.Options{
			Command: storedReq.Command,
			Workdir: workdir,
		}, func(ev exec.StreamEvent) {
			stream, err := protocol.NewStream(origID, protocol.ExecStreamEvent{
				RequestID: origID,
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
		// Record result to audit log
		a.safeExec.RecordResult(ctx, storedReq.Command, class.Level, "executed", nil, 0)
	}()
}

// execCommand runs a command through SafeExec guardrail before execution.
func (a *App) execCommand(ctx context.Context, c *websocket.Conn, env *protocol.Envelope, req protocol.ExecRequest) {
	// Build guardrail request
	grReq := guardrail.Request{
		ID:       env.ID,
		ClientID: env.ID,
		Command:  req.Command,
		Workdir:  req.Workdir,
	}

	// Resolve workdir through workspace sandbox
	workdir := req.Workdir
	if workdir != "" && a.currentWS() != nil {
		resolved, err := a.currentWS().Resolve(workdir)
		if err != nil {
			a.reply(ctx, c, protocol.NewError(env.ID, "E_SANDBOX", err.Error()))
			return
		}
		workdir = resolved
	}

	// Evaluate through guardrail
	outcome := a.safeExec.Evaluate(grReq)

	switch outcome.Decision {
	case guardrail.DecisionExecute:
		// Level 0 - auto-execute
		a.reply(ctx, c, must(protocol.NewResponse(env.ID, protocol.ExecResponse{
			Status: "ok",
			Level:  int(outcome.Class.Level),
		})))
		a.runExec(ctx, c, env.ID, req.Command, req.Shell, workdir)

	case guardrail.DecisionConfirm:
		// Level 1/2 - requires user confirmation
		a.reply(ctx, c, must(protocol.NewResponse(env.ID, protocol.ExecResponse{
			Status:  "pending",
			Level:   int(outcome.Class.Level),
			Message: fmt.Sprintf("Command requires confirmation (level: %s)", outcome.Class.Level),
		})))
		// Emit confirmation event to GUI
		ev, _ := protocol.NewEvent(protocol.ActionExecCommand, map[string]any{
			"request_id": env.ID,
			"command":    req.Command,
			"level":      int(outcome.Class.Level),
			"classification": outcome.Class.Level.String(),
		})
		a.reply(ctx, c, ev)

	case guardrail.DecisionDeny:
		// Blacklist match
		a.reply(ctx, c, must(protocol.NewResponse(env.ID, protocol.ExecResponse{
			Status:  "denied",
			Message: outcome.Message,
			Level:   int(outcome.Class.Level),
		})))

	case guardrail.DecisionCooldown:
		// Rate limited
		a.reply(ctx, c, must(protocol.NewResponse(env.ID, protocol.ExecResponse{
			Status:  "error",
			Message: outcome.Message,
		})))
	}
}

// runExec executes a command directly (after guardrail approval).
func (a *App) runExec(ctx context.Context, c *websocket.Conn, id, command, shell, workdir string) {
	_, _ = a.execEng.Run(ctx, exec.Options{
		Command: command,
		Shell:   shell,
		Workdir: workdir,
	}, func(ev exec.StreamEvent) {
		stream, err := protocol.NewStream(id, protocol.ExecStreamEvent{
			RequestID: id,
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
