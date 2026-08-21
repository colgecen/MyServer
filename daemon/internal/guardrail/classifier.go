package guardrail

import (
	"strings"
)

// Level mirrors the permission model in permissions.md:
//
//	0 READ_ONLY        – safe inspection, auto-approved
//	1 WORKSPACE_WRITE  – modifies workspace files, GUI confirmation
//	2 FULL_SYSTEM_EXEC – arbitrary/system commands, confirmation + audit
type Level int

const (
	LevelReadOnly      Level = 0
	LevelWorkspaceWrite Level = 1
	LevelFullSystem    Level = 2
)

func (l Level) String() string {
	switch l {
	case LevelReadOnly:
		return "READ_ONLY"
	case LevelWorkspaceWrite:
		return "WORKSPACE_WRITE"
	case LevelFullSystem:
		return "FULL_SYSTEM_EXEC"
	default:
		return "UNKNOWN"
	}
}

// String renders the classification for audit rows.
func (c Classification) String() string { return c.Level.String() }

// Classification is the full result of analyzing a command.
type Classification struct {
	Command string `json:"command"`
	Level   Level  `json:"level"`
	Denied  bool   `json:"denied"`
	Reason  string `json:"reason"`
}

var readOnlyCmds = map[string]bool{
	"cat": true, "less": true, "more": true, "head": true, "tail": true,
	"grep": true, "rg": true, "find": true, "stat": true, "ls": true,
	"pwd": true, "which": true, "type": true, "file": true, "wc": true,
	"uname": true, "whoami": true, "date": true, "id": true, "env": true,
	"printenv": true, "du": true, "df": true, "ps": true, "tree": true,
	"git log": true, "git status": true, "git diff": true, "git show": true,
	"git branch": true, "echo": true, "printf": true,
}

var workspaceWriteCmds = map[string]bool{
	"touch": true, "mkdir": true, "rmdir": true, "cp": true, "mv": true,
	"rm": true, "ln": true, "chmod": true, "chown": true, "sed": true,
	"awk": true, "tee": true, "truncate": true, "patch": true,
	"go build": true, "go test": true, "go run": true, "go vet": true,
	"cargo build": true, "cargo test": true, "cargo run": true,
	"npm run": true, "npm test": true, "npx": true, "yarn": true,
	"pnpm": true, "python": true, "python3": true, "node": true,
	"make": true, "cmake": true, "bash ./": false, // script exec handled below
}

var systemCmds = map[string]bool{
	"apt": true, "apt-get": true, "yum": true, "dnf": true, "pacman": true,
	"zypper": true, "brew": true, "snap": true, "flatpak": true,
	"systemctl": true, "service": true, "journalctl": true,
	"sudo": true, "su": true, "doas": true, "pkexec": true,
	"mount": true, "umount": true, "fdisk": true, "parted": true,
	"iptables": true, "nft": true, "useradd": true, "userdel": true,
	"usermod": true, "passwd": true, "crontab": true, "docker": true,
	"podman": true, "kubectl": true, "virsh": true, "modprobe": true,
	"insmod": true, "rmmod": true, "lspci": true, "lsusb": true,
}

// splitCommandChain breaks a command line on shell chaining operators
// (&&, ||, ;, |) respecting simple quoting. Each segment is classified and
// the overall level is the highest-risk component.
func splitCommandChain(cmd string) []string {
	var parts []string
	var cur strings.Builder
	inSingle, inDouble := false, false

	runes := []rune(cmd)
	for i := 0; i < len(runes); i++ {
		ch := runes[i]
		switch {
		case ch == '\'' && !inDouble:
			inSingle = !inSingle
			cur.WriteRune(ch)
		case ch == '"' && !inSingle:
			inDouble = !inDouble
			cur.WriteRune(ch)
		case !inSingle && !inDouble && ch == '&' &&
			i+1 < len(runes) && runes[i+1] == '&':
			parts = append(parts, cur.String())
			cur.Reset()
			i++ // skip second &
		case !inSingle && !inDouble && ch == '|' &&
			i+1 < len(runes) && runes[i+1] == '|':
			parts = append(parts, cur.String())
			cur.Reset()
			i++
		case !inSingle && !inDouble && (ch == ';' || ch == '|'):
			parts = append(parts, cur.String())
			cur.Reset()
		case !inSingle && !inDouble && ch == '`' || ch == '$' && i+1 < len(runes) && runes[i+1] == '(':
			// command substitution: treat as its own segment boundary marker;
			// content stays in current segment for classification
			cur.WriteRune(ch)
		default:
			cur.WriteRune(ch)
		}
	}
	parts = append(parts, cur.String())

	out := parts[:0]
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	return out
}

// baseCommand extracts the leading executable word of a command segment.
func baseCommand(segment string) string {
	fields := strings.Fields(NormalizeCommand(segment))
	if len(fields) == 0 {
		return ""
	}
	first := strings.ToLower(fields[0])
	first = strings.TrimPrefix(first, `\`)
	// strip env-var assignments prefix (FOO=bar cmd)
	for _, f := range fields {
		if strings.Contains(f, "=") && !strings.HasPrefix(f, "-") {
			continue
		}
		return strings.ToLower(f)
	}
	return first
}

// Classify analyzes a full command line. Chained commands inherit the highest
// risk of any component; sudo/su escalate to FULL_SYSTEM_EXEC; blacklist hits
// set Denied=true with the matching reason.
func (bl *Blacklist) Classify(cmd string) Classification {
	if denied, reason := bl.Check(cmd); denied {
		return Classification{Command: cmd, Level: LevelFullSystem, Denied: true, Reason: reason}
	}

	highest := LevelReadOnly
	reason := "read-only command"

	for _, seg := range splitCommandChain(cmd) {
		norm := NormalizeCommand(seg)
		fields := strings.Fields(norm)
		// skip leading env-var assignments (FOO=bar cmd ...)
		i := 0
		for i < len(fields) && strings.Contains(fields[i], "=") &&
			!strings.HasPrefix(fields[i], "-") {
			i++
		}
		base := ""
		if i < len(fields) {
			base = strings.TrimPrefix(strings.ToLower(fields[i]), `\`)
		}
		// two-word key for subcommand forms ("git status", "go build", ...)
		two := ""
		if i+1 < len(fields) {
			two = base + " " + strings.ToLower(fields[i+1])
		}
		lvl, why := classifySegment(base, two, seg)
		if lvl > highest {
			highest, reason = lvl, why
		}
	}
	return Classification{Command: cmd, Level: highest, Reason: reason}
}

func classifySegment(base, two, seg string) (Level, string) {
	switch {
	case systemCmds[base]:
		return LevelFullSystem, "system-level command: " + base
	case strings.HasPrefix(base, "/"), strings.HasSuffix(base, ".sh"):
		return LevelFullSystem, "direct script/path execution"
	case workspaceWriteCmds[two], workspaceWriteCmds[base]:
		return LevelWorkspaceWrite, "workspace write: " + base
	case readOnlyCmds[two]:
		if strings.Contains(seg, ">") {
			return LevelWorkspaceWrite, "redirected output writes files"
		}
		return LevelReadOnly, "read-only command"
	case readOnlyCmds[base]:
		// read-only base with redirect => workspace write
		if strings.Contains(seg, ">") || strings.Contains(seg, ">>") {
			return LevelWorkspaceWrite, "redirected output writes files"
		}
		return LevelReadOnly, "read-only command"
	default:
		return LevelFullSystem, "unrecognized command defaults to FULL_SYSTEM_EXEC"
	}
}
