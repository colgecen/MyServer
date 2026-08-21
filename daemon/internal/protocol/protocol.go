// Package protocol defines the JSON message envelopes exchanged between the
// MyServer GUI and the local daemon over WebSocket.
//
// The protocol is intentionally versioned and tolerant of forward-compatible
// fields: every incoming request carries a ProtocolVersion field so future
// breaking changes can be negotiated without dropping the GUI connection.
package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const ProtocolVersion = "1.0"

// MessageType discriminates the wire envelope.
type MessageType string

const (
	TypeRequest  MessageType = "request"
	TypeResponse MessageType = "response"
	TypeStream   MessageType = "stream"
	TypeEvent    MessageType = "event"
	TypeError    MessageType = "error"
)

// Action names the operation. The set is closed; unknown actions yield an error.
type Action string

const (
	ActionChat          Action = "chat"
	ActionExecCommand   Action = "exec_command"
	ActionExecApprove   Action = "exec_approve"
	ActionIndexWorkspace Action = "index_workspace"
	ActionTelemetry     Action = "telemetry"
	ActionListModels    Action = "list_models"
	ActionPullModel     Action = "pull_model"
	ActionWorkspacePick Action = "workspace_pick"
	ActionHealth        Action = "health"
)

// Envelope is the top-level wire format.
type Envelope struct {
	ID              string          `json:"id"`
	Type            MessageType     `json:"type"`
	Action          Action          `json:"action,omitempty"`
	ProtocolVersion string          `json:"protocol_version,omitempty"`
	Payload         json.RawMessage `json:"payload,omitempty"`
	Error           *ErrorPayload   `json:"error,omitempty"`
}

// ErrorPayload carries daemon-side errors back to the GUI.
type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ExecRequest is sent by the GUI to run a shell command.
type ExecRequest struct {
	Command    string `json:"command"`
	Shell      string `json:"shell,omitempty"`      // "bash" | "powershell" | "" (auto)
	Workdir    string `json:"workdir,omitempty"`
	TimeoutSec int    `json:"timeout_sec,omitempty"`
	RequestID  string `json:"request_id,omitempty"` // LLM-provided correlation id
}

// ExecResponse is the synchronous response (status only).
type ExecResponse struct {
	Status   string `json:"status"` // "ok" | "denied" | "error" | "pending"
	Level    int    `json:"level,omitempty"`
	ExitCode *int   `json:"exit_code,omitempty"`
	Duration string `json:"duration,omitempty"`
	Message  string `json:"message,omitempty"`
}

// ExecStreamEvent is streamed back while a process is running.
type ExecStreamEvent struct {
	RequestID string `json:"request_id,omitempty"`
	Stream    string `json:"stream,omitempty"` // "stdout" | "stderr" | "system"
	Data      string `json:"data,omitempty"`
	ExitCode  *int   `json:"exit_code,omitempty"`
	Done      bool   `json:"done,omitempty"`
}

// ExecApprove is sent by the GUI to confirm a pending Level 1/2 command.
type ExecApprove struct {
	RequestID string `json:"request_id"`
	Approve   bool   `json:"approve"`
}

// IndexRequest asks the daemon to (re)index a workspace path.
type IndexRequest struct {
	Path string `json:"path"`
}

// IndexResponse reports indexing progress / completion.
type IndexResponse struct {
	Status     string `json:"status"` // "ok" | "error"
	FileCount  int    `json:"file_count,omitempty"`
	ChunkCount int    `json:"chunk_count,omitempty"`
	Duration   string `json:"duration,omitempty"`
	Message    string `json:"message,omitempty"`
}

// WorkspacePickRequest is emitted when the user selects a workspace folder.
type WorkspacePickRequest struct {
	Path string `json:"path"`
}

// TelemetryEvent is broadcast periodically to all connected clients.
type TelemetryEvent struct {
	GPUUtil    float64 `json:"gpu_util"`
	VRAMUsedMB int     `json:"vram_used_mb"`
	VRAMTotal  int     `json:"vram_total_mb"`
	CPUPercent float64 `json:"cpu_percent"`
	MemPercent float64 `json:"mem_percent"`
	Timestamp  int64   `json:"timestamp"`
}

// NewRequest builds a request envelope with the protocol version stamped in.
func NewRequest(id string, action Action, payload any) (*Envelope, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}
	return &Envelope{
		ID:              id,
		Type:            TypeRequest,
		Action:          action,
		ProtocolVersion: ProtocolVersion,
		Payload:         raw,
	}, nil
}

// NewResponse builds a response envelope for a given request id.
func NewResponse(id string, payload any) (*Envelope, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}
	return &Envelope{ID: id, Type: TypeResponse, Payload: raw}, nil
}

// NewError builds an error envelope.
func NewError(id, code, msg string) *Envelope {
	return &Envelope{
		ID:    id,
		Type:  TypeError,
		Error: &ErrorPayload{Code: code, Message: msg},
	}
}

// NewStream builds a streaming event envelope.
func NewStream(id string, payload any) (*Envelope, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}
	return &Envelope{ID: id, Type: TypeStream, Payload: raw}, nil
}

// NewEvent builds a server-pushed event envelope.
func NewEvent(action Action, payload any) (*Envelope, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}
	return &Envelope{ID: nextEventID(), Type: TypeEvent, Action: action, Payload: raw}, nil
}

// Decode unmarshals the payload into v (must be a pointer).
func (e *Envelope) Decode(v any) error {
	if len(e.Payload) == 0 {
		return errors.New("empty payload")
	}
	return json.Unmarshal(e.Payload, v)
}

// Validate performs minimal sanity checks on incoming envelopes.
func (e *Envelope) Validate() error {
	if e.ID == "" {
		return errors.New("missing id")
	}
	if e.Type == "" {
		return errors.New("missing type")
	}
	if e.Type == TypeRequest && e.Action == "" {
		return errors.New("request missing action")
	}
	return nil
}

var evtCounter int64

func nextEventID() string {
	// monotonic-ish; avoids pulling in google/uuid for a non-critical id.
	return fmt.Sprintf("evt-%d-%d", time.Now().UnixNano(), evtCounter)
}
