# Security Architecture & Execution Guardrails

## Connection Restrictions

All execution and LLM communication is strictly confined to **localhost only**:

```
- Daemon HTTP/WebSocket: 127.0.0.1:4096 (GUI ↔ Daemon)
- Ollama API: 127.0.0.1:11434 (LLM inference)
- Telemetry Endpoint: 127.0.0.1:4096/api/telemetry
```

Remote connections (external IPs) are: 
- Blocked at firewall-level by the SafeExec daemon
- Logged as critical audit events

---

## Regex Blacklist Implementation

The SafeExec module uses this regex pattern to match forbidden commands before execution:

```regex
^(rm|del|format|mkfs|dd|shutdown|init|reboot|chmod|chown|reg|sysctl).*$\n```

### Matched Command Categories

| Command Pattern | Risk Level | Action on Match |
|-----------------|------------|----------------|
| `rm -rf /`      | Level 0    | Immediate Denial |
| `chmod -R 777` | Level 1    | GUI Confirmation |
| `shutdown -h`   | Level 2    | GUI Confirmation + Audit |
| `dd if=/dev/zero` | Level 2  | GUI Confirmation |
| `reg delete HKLM` | Level 2  | GUI Confirmation |

All Level 1/2 commands require pre-validation through the GUI confirmation dialog.

---

## Execution Policy

### Level System

```
[0] READ_ONLY    : Read-only filesystem operations
[1] WORKSPACE_WRITE : File modifications in workspace directory
[2] FULL_SYSTEM_EXEC : OS-level commands affecting system partitions
```

### Mandatory Confirmation Workflow
1. When a Level 1/2 command is proposed:
   ```
   - GUI pops confirmation dialog: "Execute: [COMMAND]?"
   - User must click [CONFIRM] or [CANCEL]
   - Denied commands trigger audit log entry
```

2. Dry-run mode available for preview:
   ```
   - Command output simulated in terminal preview pane
   - No actual filesystem changes occur
```

### Audit Logging

Every command submission is logged in the audit system:
```
Type: ExecAudit
{
  "command": "rm -rf /home/user/old",
  "classification": "WORKSPACE_WRITE",
  "user": "colgecen",
  "timestamp": "2026-08-21T14:35:22Z",
  "approved": true,
  "exit_code": 0,
  "duration_ms": 42
}
```

---

## Bypass Prevention

- Regex patterns updated monthly via config file
- All system commands pass through 3 validation stages:
  1. Regex blacklist
  2. Command classification
  3. User permission check
- Dry-run mode prevents accidental destructive actions