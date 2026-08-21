# MyServer Architecture Specification

## System Overview

MyServer is a desktop AI cockpit application that provides a cyberpunk-themed GUI for local LLM interaction with full system execution capabilities. The system follows a **client-daemon architecture**: a Tauri-based frontend communicates with a local Go daemon over WebSocket (127.0.0.1:4096), which in turn orchestrates a local LLM (Ollama/llama.cpp on 127.0.0.1:11434) and manages command execution with multi-layer security guardrails.

**Version:** v1.14.33  
**Architecture:** Client-Daemon (Desktop + Local Background Service)  
**Communication Protocol:** JSON-over-WebSocket (primary), HTTP/JSON (secondary)

---

## Component Architecture

### 1. Frontend / GUI (Client)

```
┌─────────────────────────────────────────────────────────┐
│                    MyServer GUI                         │
│              (Tauri v2 + React 18 + TypeScript)         │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  [Header Bar]                                           │
│    ├── Local Server v1.14.33 [ACTIVE] indicator        │
│    ├── Model Selector (dropdown)                       │
│    └── GPU/VRAM telemetry (live canvas chart)           │
│                                                         │
│  [Left Sidebar]                                         │
│    ├── Chat History (session list)                      │
│    ├── Workspace Selector (folder picker)               │
│    └── Model Downloader (Ollama API)                    │
│                                                         │
│  [Main Canvas]                                          │
│    ├── Reasoning Panel (AI thinking process)            │
│    ├── Code/Command Block (syntax highlighted)          │
│    └── [RUN COMMAND] button (confirmation)              │
│                                                         │
│  [Input HUD]                                            │
│    ├── Oval glowing prompt bar                         │
│    ├── File/Folder attach button                        │
│    └── Terminal Exec shortcut                           │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

**Responsibilities:**
- Render cyberpunk design system (glassmorphism, neon accents)
- Manage WebSocket connection to daemon
- Handle user input (prompt, file selection, command approval)
- Parse and display LLM streaming responses
- Show real-time GPU/VRAM telemetry via daemon API
- Enforce UI-level validation before sending commands

**Technology Stack:**
- **Runtime:** Tauri v2 (Rust-based desktop shell)
- **UI Library:** React 18 + TypeScript
- **Styling:** CSS-in-JS with glassmorphism effects
- **Charts:** Canvas-based GPU/VRAM telemetry
- **IPC:** WebSocket to `127.0.0.1:4096`

### 2. Daemon / Backend (Local Server)

```
┌─────────────────────────────────────────────────────────┐
│                  MyServer Daemon                       │
│              (Go 1.22+, HTTP/WebSocket)                │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  [HTTP Router]                                          │
│    ├── /api/health                                     │
│    ├── /api/workspace                                  │
│    ├── /api/exec                                       │
│    ├── /api/models                                     │
│    ├── /api/llm                                          │
│    └── /api/telemetry                                  │
│                                                         │
│  [WebSocket Handler]                                    │
│    ├── JSON-RPC command streaming                      │
│    ├── Event broadcast (telemetry, LLM stream)         │
│    └── Session management                              │
│                                                         │
│  [Workspace Indexer]         ←──┐                        │
│    ├── Directory walker         │                        │
│    ├── File type detection      │                        │
│    ├── Embedding pipeline       │                        │
│    └── Context window manager   │                        │
│                                                         │
│  [SafeExec Module]                                    │
│    ├── Forbidden command regex filter                │
│    ├── Command classification (Level 0/1/2)          │
│    ├── User confirmation gate                         │
│    ├── Audit logging                                   │
│    └── Dry-run preview                                 │
│                                                         │
│  [LLM Orchestrator]                                   │
│    ├── Ollama API client (127.0.0.1:11434)            │
│    ├── Master System Prompt injection                 │
│    ├── Reasoning parser (thinking tags)               │
│    ├── exec_shell directive parser                    │
│    └── Response streaming                             │
│                                                         │
│  [Telemetry Agent]                                    │
│    ├── GPU stats (NVIDIA-SMI / AMD)                   │
│    ├── VRAM usage                                     │
│    └── System metrics (CPU, mem, disk)                 │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

**Responsibilities:**
- Listen on `127.0.0.1:4096` for GUI connections
- Route commands through SafeExec before shell execution
- Index workspace directory and inject context to LLM
- Communicate with local LLM via Ollama API (`127.0.0.1:11434`)
- Stream telemetry data to GUI
- Maintain audit logs and enforce security policies

**Technology Stack:**
- **Language:** Go 1.22+ (compiled binary)
- **Web Framework:** Fiber or Gin (for HTTP) + nhooyr.io/websocket
- **Shell Execution:** `os/exec` with process groups for graceful termination
- **Embeddings:** On-demand via Ollama API
- **Storage:** BoltDB (embedded KV store for workspace index, audit log)

### 3. Local LLM (Ollama / llama.cpp)

```
┌─────────────────────────────────────────────────────────┐
│                Local LLM Engine                        │
│              (Ollama API on 127.0.0.1:11434)            │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  [Model Registry]                                       │
│    ├── Available models (quantized local)              │
│    ├── Download queue (from registry)                 │
│    └── Quantization settings (Q4_K_M, Q8_0, etc.)       │
│                                                         │
│  [Chat Completion API]                                  │
│    ├── Master System Prompt (injected per request)      │
│    ├── Workspace context (from daemon)                  │
│    ├── Tool definitions (exec_shell schema)             │
│    └── Streaming responses                              │
│                                                         │
│  [Embedding API]                                        │
│    ├── Text embedding (for workspace indexing)          │
│    ├── Similarity search                               │
│    └── Reranking                                          │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

**Responsibilities:**
- Host quantized models (LLaVA, DeepSeek-Coder, Qwen2, etc.)
- Serve chat completion API with streaming
- Provide embedding API for workspace context injection
- Accept Master System Prompt via system parameter

**Technology Stack:**
- **Backend:** Ollama (Go-based LLM runner)
- **Models:** GGUF quantized models (CPU/GPU inference)
- **API:** HTTP/JSON (Ollama-compatible at 127.0.0.1:11434)

---

## Architecture Data Flow

### ASCII Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              MYDEVICE (User Machine)                        │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  ┌──────────────────────────────────────────────────────────────┐   │   │
│  │  │                    MYOS SERVER GUI                           │   │   │
│  │  │         (Tauri + React 18 + TypeScript)                      │   │   │
│  │  │                                                              │   │   │
│  │  │  [Header Bar]         [Left Sidebar]        [Main Canvas]    │   │   │
│  │  │  [Input HUD]                                    [RUN CMD]     │   │   │
│  │  └────────────────────────┬──────────────────────────────────────┘   │   │
│  │                           │ WebSocket JSON                           │   │
│  │                           │ 127.0.0.1:4096                           │   │
│  │  ┌────────────────────────▼──────────────────────────────────────┐   │   │
│  │  │                    MYOS DAEMON                               │   │   │
│  │  │         (Go 1.22+ — Local HTTP/WebSocket Server)             │   │   │
│  │  │                                                              │   │   │
│  │  │  ┌─────────────────┐  ┌──────────────────────┐              │   │   │
│  │  │  │ Workspace Index │  │   SafeExec Module    │              │   │   │
│  │  │  │  ─ Directory    │  │  ─ Regex Filter      │              │   │   │
│  │  │  │    Walk        │  │  ─ Command Class.    │              │   │   │
│  │  │  │  ─ File Type    │  │  ─ User Confirmation │              │   │   │
│  │  │  │    Detection   │  │  ─ Audit Log         │              │   │   │
│  │  │  │  ─ Embeddings   │  │  ─ Dry-Run Preview   │              │   │   │
│  │  │  │  ─ Context Win. │  │                      │              │   │   │
│  │  └──────────┬──────────┘  └──────────┬─────────────┘              │   │   │
│  │             │                        │                              │   │   │
│  │  ┌──────────▼────────────────────────▼─────────────┐              │   │   │
│  │  │         LLM ORCHESTRATOR                          │              │   │   │
│  │  │  ─ Ollama API Client (127.0.0.1:11434)            │              │   │   │
│  │  │  ─ Master System Prompt Injection                 │              │   │   │
│  │  │  ─ Reasoning Parser (thinking tags)               │              │   │   │
│  │  │  ─ exec_shell Directive Parser                   │              │   │   │
│  │  │  ─ Response Streaming                              │              │   │   │
│  │  └────────────────────────┬─────────────────────────┘              │   │   │
│  │                           │ HTTP/JSON                                    │   │
│  │                           │ 127.0.0.1:11434                              │   │
│  │  ┌────────────────────────▼──────────────────────────────────────┐   │   │
│  │  │                    LOCAL LLM ENGINE                           │   │   │
│  │  │         (Ollama — Quantized Models, CPU/GPU)                  │   │   │
│  │  │                                                               │   │   │
│  │  │  [Model Registry]  /api/pull, /api/list, /api/delete          │   │   │
│  │  │                                                               │   │   │
│  │  │  [Chat Completions] /api/chat                                  │   │   │
│  │  │                                                               │   │   │
│  │  │  [Embeddings] /api/embed                                     │   │   │
│  │  └───────────────────────────────────────────────────────────────┘   │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│  [OS Shell] ────────────────────────────────── (Bash / PowerShell)         │
│     │                                                                        │
│  ┌──▼──────────────────────────────────────────────────────────────────────┐│
│  │  Local Execution Layer (os/exec, process groups)                       ││
│  └────────────────────────────────────────────────────────────────────────┘│
│                                                                             │
│  [File System] ────────────────────────────────── (Workspace Directory)   │
│                                                                             │
│  [Hardware] ───────────────────────────────────── (GPU, CPU, VRAM, Disk)    │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Request Flow: Chat Prompt → LLM Response → Exec

```
User types prompt in GUI
    │
    ▼
GUI sends {action: "chat", prompt, workspace_files} via WebSocket
    │
    ▼
Daemon receives → injects Workspace Context + Master System Prompt
    │
    ▼
Daemon forwards to LLM (Ollama /chat/completions, streaming)
    │
    ▼
LLM returns reasoning tokens + exec_shell directive
    │
    ▼
Daemon parses exec_shell directive → extracts command
    │
    ▼
SafeExec Module runs command through Regex Blacklist
    │
    ├── MATCH → REJECT → Send "SECURITY ALERT" to GUI
    │
    └── NO MATCH → Classify Level → Level 1/2 requires GUI confirmation
                                            │
                                            ▼
                                    User clicks [RUN COMMAND]
                                            │
                                            ▼
                                    Daemon executes via os/exec (Bash/Pwsh)
                                            │
                                            ▼
                                    Output streamed back to GUI + LLM
```

### Request Flow: Workspace Selection → Context Injection

```
User selects folder in Workspace Selector
    │
    ▼
GUI sends {action: "index_workspace", path: "/home/user/project"} via WebSocket
    │
    ▼
Daemon walks directory tree → filters by .gitignore + file types
    │
    ▼
For each supported file:
    ├── Read content
    ├── Chunk (AST-based, ~500 tokens per chunk)
    ├── Generate embedding (Ollama /api/embed)
    └── Store in BoltDB index
    │
    ▼
Daemon returns {status: "indexed", file_count, total_tokens} to GUI
    │
    ▼
On next chat:
    ├── Retrieve top-K similar chunks (vector similarity)
    ├── Inject into context window (token budgeted)
    └── Send enriched prompt to LLM
```

### Request Flow: Command Execution (SafeExec)

```
LLM produces: ```exec_shell rm -rf /home/user/old_project```
    │
    ▼
Daemon extracts: "rm -rf /home/user/old_project"
    │
    ▼
SafeExec.run():
    1. Match against ForbiddenCommands regex list
       ├── rm -rf / (Level 0 — DENY) → "rm -rf /" matches "rm -rf /" pattern
       │
    2. No match → classify_command():
       ├── Destructive keywords (rm, del, format, etc.) → Level 1 (WORKSPACE_WRITE)
       │
    3. Level 1 → emit {action: "exec_request", command, level} to GUI
    │
    ▼
GUI shows confirmation dialog: "Execute: rm -rf /home/user/old_project"
    │
    ▼
User clicks "Confirm"
    │
    ▼
Daemon.execute():
    ├── Log to audit log (command, user, timestamp, risk_level)
    ├── Spawn process with os/exec (process group for cleanup)
    ├── Stream stdout/stderr back to GUI
    ├── Return exit code + duration
    └── Clean up process group
```

---

## Security Architecture

### Trust Boundaries

```
┌─────────────────────────────────────────────┐
│  GUI (Tauri Renderer)                        │
│  ──── User Trust Boundary ────              │
│  ┌─────────────────────────────────────┐   │
│  │  User clicks [RUN COMMAND]          │   │
│  │  User selects workspace folder      │   │
│  └─────────────────────────────────────┘   │
│                │                           │
│                ▼ WebSocket (localhost)     │
│  ┌─────────────────────────────────────┐   │
│  │  GUARDRAIL BOUNDARY (Daemon)        │   │
│  │  ── SafeExec Module must approve ── │   │
│  └─────────────────────────────────────┘   │
│                │                           │
│                ▼ os/exec (if approved)     │
│  ┌─────────────────────────────────────┐   │
│  │  OS Shell (Bash/PowerShell)         │   │
│  │  ──── System Trust Boundary ────   │   │
│  └─────────────────────────────────────┘   │
└─────────────────────────────────────────────┘
```

### Network Topology

| Service | Address | Purpose | Exposed to Network |
|---------|---------|---------|-------------------|
| MyServer Daemon | `127.0.0.1:4096` | GUI ↔ Daemon communication | **NO** (localhost only) |
| Ollama API | `127.0.0.1:11434` | LLM inference | **NO** (localhost only) |
| Telemetry API | `127.0.0.1:4096/api/telemetry` | GPU/system stats | **NO** (localhost only) |

---

## Data Models

### WebSocket Message Protocol (Daemon ↔ GUI)

```typescript
// Request from GUI to Daemon
interface WSRequest {
  id: string;            // UUID for request tracking
  action: string;        // "chat", "exec_command", "index_workspace",
                          // "get_models", "pull_model", "telemetry_subscribe"
  payload: Record<string, any>;
}

// Response from Daemon to GUI
interface WSResponse {
  id: string;            // matches request id
  type: "response" | "stream" | "error" | "event";
  payload: {
    status?: "ok" | "error";
    data?: any;
    stream?: boolean;     // true for streaming responses
  };
}

// Streaming event payloads
interface ExecStreamEvent {
  stdout?: string;
  stderr?: string;
  exit_code?: number;
  done?: boolean;
}

interface TelemetryEvent {
  gpu_util?: number;
  vram_used_mb?: number;
  vram_total_mb?: number;
  cpu_percent?: number;
  memory_percent?: number;
}
```

### Audit Log Entry (BoltDB)

```go
type AuditLogEntry struct {
    ID          string    `json:"id"`
    Timestamp   time.Time `json:"timestamp"`
    Command     string    `json:"command"`
    Classification ExecLevel `json:"classification"`  // LEVEL_0, LEVEL_1, LEVEL_2
    ApprovedBy  string    `json:"approved_by"`        // "user" or "auto" or "denied"
    Status      string    `json:"status"`             // "executed", "denied", "skipped"
    ExitCode    *int      `json:"exit_code,omitempty"`
    Duration    string    `json:"duration"`
}
```

### Workspace Index Entry (BoltDB)

```go
type WorkspaceEntry struct {
    Path        string   `json:"path"`
    Language    string   `json:"language"`        // "go", "python", "rust", etc.
    ContentHash string   `json:"content_hash"`
    ChunkCount  int      `json:"chunk_count"`
    LastIndexed time.Time `json:"last_indexed"`
}

type IndexedChunk struct {
    ID            string    `json:"id"`
    FilePath      string    `json:"file_path"`
    StartLine     int       `json:"start_line"`
    EndLine       int       `json:"end_line"`
    Content       string    `json:"content"`
    Embedding     []float32 `json:"embedding"`    // 768-dim vector
}
```

---

## Deployment Topology

### Bundled Distribution

```
myserver/                          # Tauri app bundle
├── myserver.exe                  # Tauri main process (Rust)
├── myserver-daemon               # Go daemon (sidecar binary)
├── ollama/                       # Embedded Ollama (optional)
│   └── models/                   # Quantized model files (.gguf)
├── resources/
│   ├── ui/                      # React build output
│   └── assets/
└── MyServer.app (macOS) / myserver.msi (Windows) / myserver.deb (Linux)
```

### Runtime Process Tree

```
myserver (Tauri main process)
├── myserver-daemon (Go subprocess)
│   ├── HTTP server on 127.0.0.1:4096
│   ├── WebSocket handler
│   └── SafeExec engine
├── ollama (if embedded)
│   └── HTTP server on 127.0.0.1:11434
└── Renderer process (React UI)
    └── WebSocket client → 127.0.0.1:4096
```

---

## Performance & Scalability Considerations

| Component | Bottleneck | Mitigation |
|-----------|-----------|------------|
| Workspace indexing | Large file trees | Parallel directory walk, progress streaming, incremental indexing |
| Context injection | LLM context window limit | Token budget manager, sliding window, selective chunk retrieval |
| Command execution | Long-running processes | Non-blocking os/exec with goroutine-per-command, stream output |
| LLM inference | Single-threaded Ollama | GPU offloading (CUDA/Metal), KV cache optimization |
| Telemetry polling | High-frequency updates | 1s polling interval, delta-based updates only |

---

## Versioning & Compatibility

- **Daemon API:** Versioned via `/api/v1/` prefix; backward compatible within major version
- **WebSocket Protocol:** Versioned via `protocol_version` field in handshake message
- **Model Compatibility:** Supports any GGUF-quantized model; Ollama-native models recommended
- **Frontend Compatibility:** Tauri v2 minimum; requires OpenGL 3.3+ for glassmorphism effects