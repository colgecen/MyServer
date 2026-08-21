# MyServer Daemon (myserverd)

Local HTTP/WebSocket daemon for MyServer v1.14.33.

Listens on `127.0.0.1:4096` and exposes:

- HTTP `/api/health`
- HTTP `/api/telemetry`
- WebSocket `/ws` (JSON-RPC over WS)

## Build

```bash
cd daemon
go mod tidy
go build -o bin/myserverd ./cmd/myserverd
```

## Run

```bash
./bin/myserverd
```

## Layout

```
cmd/myserverd     - main entrypoint
internal/server   - HTTP + WebSocket server
internal/protocol - JSON message types
internal/exec     - command execution pipeline (Bash/PowerShell)
internal/workspace - workspace path validation & sandboxing
internal/indexer  - recursive walker, language detection, chunking, embeddings
internal/guardrail- regex blacklist & command classification
internal/telemetry - GPU/VRAM/CPU stats
internal/store    - BoltDB-backed audit + index storage
pkg/api           - public API types
configs           - default config + blacklist patterns
```
