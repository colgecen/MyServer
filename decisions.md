# Architecture Decision Records (ADRs)

---

## ADR-001: Local-First Daemon Architecture

### Status: Accepted  
### Date: 2026-08-21  
### Deciders: System Architect, Lead Engineer

### Context
The application requires full local system access (file reading, command execution, hardware monitoring) while maintaining strict security boundaries. A cloud-based architecture would introduce latency, privacy risks, and dependency on external services.

### Decision
We will implement a **local daemon architecture**:
- A Go daemon running on `127.0.0.1:4096` as the central execution hub
- WebSocket-based JSON protocol for frontend communication
- All system access mediated through the daemon (no direct OS access from GUI)
- Ollama API on `127.0.0.1:11434` for local LLM inference

### Consequences
**Positive:**
- Full offline operation — no external dependencies
- Low-latency local communication (sub-millisecond IPC)
- Strict security boundary enforcement at daemon level
- Hardware telemetry accessible via local API

**Negative:**
- Increased process overhead (daemon + LLM + GUI)
- Requires Go build toolchain for daemon compilation
- Model files consume local disk space (1-8 GB per model)

### Alternatives Considered
1. **Cloud LLM + Cloud Execution** — Rejected: privacy, latency, offline requirement
2. **Single-process Electron app** — Rejected: security boundary blur, heavy process
3. **Browser-based desktop** — Rejected: sandbox restrictions prevent system access

---

## ADR-002: Regex-Based Security Filter

### Status: Accepted  
### Date: 2026-08-21  
### Deciders: Security Engineer, System Architect

### Context
The system must prevent execution of destructive commands while allowing legitimate system administration. A pure allowlist approach is too restrictive for user workflows; a denylist approach with clear patterns provides flexibility while blocking known-dangerous operations.

### Decision
We will implement a **multi-layered security filter**:
1. **Regex Blacklist** — Pattern matching against known destructive commands
2. **Command Classification** — Automatic risk level assignment (Level 0/1/2)
3. **Mandatory Confirmation** — GUI dialog required for Level 1/2 commands
4. **Audit Logging** — All executions recorded with timestamp, command, user, result

### Regex Blacklist Table
| Pattern | Risk | Action |
|---------|------|--------|
| `rm -rf /` | 0 | Deny |
| `rm -rf /*` | 0 | Deny |
| `del /f /s /q C:\*` | 0 | Deny |
| `rd /s /q c:\` | 0 | Deny |
| `mkfs.*` | 0 | Deny |
| `format c:` | 0 | Deny |
| `dd if=/dev/zero of=/dev/sd*` | 0 | Deny |
| `shutdown -h now` | 0 | Deny |
| `init 0` | 0 | Deny |
| `reboot -f` | 0 | Deny |
| `chmod -R 777 /` | 0 | Deny |
| `chown -R root /` | 0 | Deny |
| `reg delete HKLM\System` | 0 | Deny |
| `sysctl -w kernel.*` | 0 | Deny |

### Consequences
**Positive:**
- Clear, auditable security policy
- Easy to update blacklist patterns
- User-friendly confirmation dialogs
- Full audit trail for compliance

**Negative:**
- Regex patterns require regular maintenance
- Potential bypass via command obfuscation (e.g., variable expansion, alias)
- User friction for legitimate admin tasks

### Mitigations
- Monthly security review of blacklist patterns
- Shell metacharacter parsing to prevent obfuscation
- Dry-run mode for preview before execution
- Audit log review recommendations

### Alternatives Considered
1. **Pure Allowlist** — Rejected: too restrictive for user workflows
2. **AI-based classification** — Rejected: non-deterministic, unpredictable
3. **Sandboxed execution (containers)** — Rejected: heavy setup, limited functionality

---

## ADR-003: Cyberpunk Design System

### Status: Accepted  
### Date: 2026-08-21  
### Deciders: UI/UX Designer, Frontend Lead

### Context
The application targets developers and system administrators who need a focused, high-density interface. A cyberpunk aesthetic with glassmorphism provides visual distinction while maintaining readability during long sessions.

### Decision
We will implement a **cyberpunk minimalist design system**:
- Background: `#05070A` (Deep Void Black)
- Panels/Cards: `#0B111E` at 80% opacity with 12px blur (glassmorphism)
- Accent colors: `#00F0FF` (Cyber Cyan), `#0066FF` (Electric Blue)
- Text: `#FFFFFF` (primary), `#E2E8F0` (secondary)
- Borders: 1px solid `rgba(0, 240, 255, 0.15)` (Neon Aura)

### Design Tokens
```css
:root {
  --bg-deep: #05070A;
  --panel-glass: rgba(11, 17, 30, 0.80);
  --accent-cyan: #00F0FF;
  --accent-blue: #0066FF;
  --text-primary: #FFFFFF;
  --text-secondary: #E2E8F0;
  --border-neon: rgba(0, 240, 255, 0.15);
  --blur-amount: 12px;
}
```

### Consequences
**Positive:**
- Distinctive visual identity in a crowded desktop app market
- Glassmorphism improves visual hierarchy and depth perception
- Dark theme reduces eye strain during extended use
- Neon accents guide attention to key interactive elements

**Negative:**
- Requires WebGL/GPU support for blur effects
- May not meet WCAG contrast requirements in all states
- Potential performance impact on lower-end hardware

### Mitigations
- Fallback to flat background if GPU acceleration unavailable
- Contrast checker integrated into CI pipeline
- Hardware acceleration toggle in settings

### Alternatives Considered
1. **Material Design (Light)** — Rejected: eye strain for long sessions
2. **Flat Minimalist** — Rejected: lacks distinctive identity
3. **High-Contrast Dark** — Rejected: too clinical, no visual depth

---

## ADR-004: Go Daemon with WebSocket Protocol

### Status: Accepted  
### Date: 2026-08-21  
### Deciders: Backend Lead, System Architect

### Context
The daemon needs to handle concurrent operations (LLM streaming, command execution, telemetry polling) while maintaining low latency. HTTP polling introduces unnecessary overhead and complexity.

### Decision
We will use **WebSocket as the primary communication protocol** between GUI and daemon:
- Full-duplex communication for streaming LLM responses
- JSON message format for structured data
- Connection management with automatic reconnection
- Heartbeat mechanism for connection health monitoring

### Protocol Design
```json
{
  "id": "req-uuid",
  "action": "chat|exec_command|index_workspace|telemetry",
  "payload": { ... },
  "protocol_version": "1.0"
}
```

### Consequences
**Positive:**
- Real-time streaming of LLM responses and command output
- Single connection for all operations
- Low overhead compared to HTTP polling
- Natural fit for event-driven architecture

**Negative:**
- Requires WebSocket server implementation
- Connection state management complexity
- Need for fallback mechanisms on connection loss

### Alternatives Considered
1. **HTTP Long-Polling** — Rejected: higher latency, less efficient
2. **gRPC** — Rejected: heavier dependency, browser compatibility issues
3. **MQTT** — Rejected: over-engineering for local communication

---

## ADR-005: Tauri Desktop Framework

### Status: Accepted  
### Date: 2026-08-21  
### Deciders: Frontend Lead, System Architect

### Context
The application needs a desktop shell that can bundle a Go daemon, React UI, and system binaries into a single distributable package. Electron provides this but with a heavy memory footprint.

### Decision
We will use **Tauri v2** as the desktop framework:
- Rust-based main process (small binary, ~5MB)
- React 18 + TypeScript for UI
- Native system tray and menu integration
- Bundled Go daemon as sidecar binary
- Cross-platform: Windows, macOS, Linux

### Consequences
**Positive:**
- Small binary size compared to Electron
- Native performance for system-level operations
- Strong security model (isolated renderer)
- Built-in updater and signing support

**Negative:**
- Requires Rust toolchain for development
- Smaller community than Electron
- Webview rendering differences across platforms

### Alternatives Considered
1. **Electron** — Rejected: 80-100MB+ binary, high memory usage
2. **Flutter** — Rejected: different language, less flexible for system integration
3. **NW.js** — Rejected: outdated, poor performance