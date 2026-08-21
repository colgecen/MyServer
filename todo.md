# MyServer Development Todo List

## Phase 1: Core Daemon & Execution Engine
- [x] Initialize Go/Node.js daemon project structure
- [x] Implement HTTP/WebSocket server on 127.0.0.1:4096
- [x] Build command execution pipeline (Bash/PowerShell)
- [x] Implement request/response protocol (JSON over WebSocket)
- [x] Add process management (spawn, monitor, kill, stdout/stderr streaming)
- [x] Implement workspace path validation and sandboxing
- [x] Add daemon health check and auto-restart logic
- [x] Write integration tests for command execution flow

## Phase 2: Workspace Indexer & Context Injection
- [x] Implement recursive directory walker with ignore patterns (.gitignore, node_modules, etc.)
- [x] Build file type detection and language mapping
- [x] Create semantic code chunking (AST-based for supported languages)
- [x] Implement vector embedding pipeline (local embeddings via Ollama/llama.cpp)
- [x] Build context window management (token budgeting, sliding window)
- [x] Add file watcher for live index updates
- [x] Implement workspace selection IPC (Frontend ↔ Daemon)
- [x] Write unit tests for indexer accuracy and performance

## Phase 3: Security Guardrails & SafeExec Module
- [x] Implement Regex-based Forbidden Command List (Blacklist Engine)
- [x] Build command classification: READ_ONLY / WORKSPACE_WRITE / FULL_SYSTEM_EXEC
- [x] Add mandatory user confirmation UI for Level 1 & 2 commands
- [x] Implement audit logging (command, user, timestamp, result, risk level)
- [x] Add rate limiting and anomaly detection for exec requests
- [x] Build safe environment variable sanitization
- [x] Implement dry-run mode for command preview
- [x] Write security test suite (penetration attempts, bypass vectors)

## Phase 4: Cyberpunk UI / Frontend (Tauri + React)
- [x] Set up Tauri v2 project with React 18 + TypeScript
- [x] Implement design system: colors, glassmorphism, neon accents, typography
- [x] Build Header Bar: version badge, model selector, GPU/VRAM telemetry charts
- [x] Build Left Sidebar: chat history, workspace selector, model downloader
- [x] Build Main Canvas: reasoning panel, code/command blocks, RUN COMMAND button
- [x] Build Input HUD: oval prompt bar, file/folder attach, terminal exec shortcut
- [x] Implement WebSocket connection to daemon with reconnection logic
- [x] Add keyboard shortcuts (Cmd/Ctrl+K, Cmd/Ctrl+Enter, etc.)
- [x] Implement theme persistence and accessibility (WCAG AA)
- [x] Write E2E tests for critical user flows

## Phase 5: Local LLM Integration
- [x] Integrate Ollama API client (model pull, list, chat, embeddings)
- [x] Implement Master System Prompt injection
- [x] Build reasoning parser (thinking tags, tool calls, final output)
- [x] Add streaming response rendering in UI
- [x] Implement model management UI (download, switch, delete, quantize)

## Phase 6: Packaging & Release
- [x] Build Tauri sidecar for daemon bundling
- [x] Create installer packages: .deb, .rpm, .AppImage, .msi, .dmg
- [x] Implement auto-update mechanism (Tauri Updater)
- [x] Write release automation (GitHub Actions, signing, notarization)
- [x] Final integration testing and performance profiling

## Phase 7: Telemetry & Observability
- [x] Implement GPU/CPU/VRAM collectors (NVIDIA-SMI, sysfs fallback)
- [x] Build 1s polling loop with delta-based broadcast
- [x] Add WebSocket telemetry event via protocol/telemetry
- [x] Create ring buffer for telemetry history (last 300 samples)
- [x] Build frontend live telemetry charts integration
- [x] Write benchmarks for collector performance

## Phase 8: Persistent Store & Session Memory
- [x] Design BoltDB schema for workspace/cache/audit
- [x] Implement KV store wrapper with transactions and TTL
- [x] Add chat session persistence (history, workspace binding)
- [x] Implement workspace snapshot and restore
- [x] Write store integration tests (crash recovery, concurrent writes)

## Phase 9: Autonomous Agent & Tool Router
- [x] Define tool schema (exec_shell, read_file, search_index)
- [x] Build agent loop with LLM tool calling
- [x] Implement MCP-style tool registry and permission gate
- [x] Add multi-step plan executor with guardrail gating
- [ ] Build agent timeline UI (step list, tool outputs)
- [ ] Write agent E2E simulation tests

## Phase 10: Hardening & Compliance
- [ ] Implement RBAC and API token authentication
- [ ] Add secrets redaction in logs and audit trails
- [ ] Build signed audit trail (HMAC per entry)
- [ ] Add compliance export (JSON/SIEM format)
- [ ] Final chaos and recovery testing

---

## Workflow Rule: Mandatory Git Commits

**RULE:** Every task completion (checkbox marked) MUST be followed by a Git commit using Conventional Commits format.

### Commit Format
```
<type>(<scope>): <imperative summary>

[optional body]

[optional footer]
```

### Type Mapping per Phase
| Phase | Scope | Example Commit |
|-------|-------|----------------|
| 1 | daemon | `feat(daemon): implement websocket command execution pipeline` |
| 1 | daemon | `fix(daemon): handle stderr streaming for long-running processes` |
| 2 | indexer | `feat(indexer): add AST-based chunking for Go/Python/Rust` |
| 2 | indexer | `perf(indexer): optimize file walker with parallel traversal` |
| 3 | guardrail | `security(guardrail): implement regex blacklist for destructive commands` |
| 3 | guardrail | `feat(guardrail): add mandatory user confirmation for Level 2 exec` |
| 4 | ui | `feat(ui): build cyberpunk glassmorphism design system` |
| 4 | ui | `feat(ui): implement reasoning panel with streaming output` |
| 5 | llm | `feat(llm): integrate ollama client with master system prompt` |
| 6 | pkg | `chore(release): package v1.14.33 for linux/windows/macos` |
| 7 | telemetry | `feat(telemetry): implement GPU/VRAM collectors with polling` |
| 8 | store | `feat(store): add BoltDB KV wrapper with TTL` |
| 9 | agent | `feat(agent): build tool router with guardrail gating` |
| 10 | security | `security(auth): implement RBAC and token auth` |

### Commit Rules
1. **One logical change per commit** — don't bundle unrelated changes
2. **Imperative mood** — "add" not "added", "fix" not "fixed"
3. **Scope in parentheses** — matches the component being modified
4. **Reference issues/tasks** — add `Refs: #TASK-ID` in footer when applicable
5. **Sign commits** — use `git commit -s` for DCO compliance

### Enforcement
- Pre-commit hook validates commit message format
- CI pipeline rejects non-conforming commits
- Todo checkboxes auto-uncheck if commit missing (via git hook)