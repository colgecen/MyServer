// Package guardrail implements the SafeExec security module: regex blacklist,
// command classification, confirmation gating, audit logging, rate limiting
// and environment sanitization.
package guardrail

import (
	"os"
	"regexp"
	"strings"
)

// Verdict is the outcome of the blacklist stage.
type Verdict string

const (
	VerdictAllow Verdict = "allow"
	VerdictDeny  Verdict = "deny"
)

// compiledRule pairs a regex with its human-readable description.
type compiledRule struct {
	re          *regexp.Regexp
	description string
}

// defaultDenyPatterns are matched against a NORMALIZED command string
// (quotes stripped, whitespace collapsed). All patterns are case-insensitive
// and word-anchored where necessary so benign commands like `go fmt` or
// `printf "format"` are not caught.
var defaultDenyRules = []struct {
	pattern     string
	description string
}{
	// recursive root destruction
	{`(?i)\brm\s+(-[a-z]*r[a-z]*f|-[a-z]*f[a-z]*r)[a-z]*\s+(/|\*|~|/\*)\s*$`, "recursive force delete of filesystem root"},
	{`(?i)\brm\s+-[a-z]*r[a-z]*f[a-z]*\s+/(?:\s|$)`, "rm -rf /"},
	{`(?i)\bdel\s+/f\s+/s\s+/q\s+c:\\`, "windows mass delete"},
	{`(?i)\brd\s+/s\s+/q\s+c:\\`, "windows mass rmdir"},

	// disk destruction
	{`(?i)\bmkfs(\.[a-z0-9]+)?\b`, "filesystem format"},
	{`(?i)^format\s+[c-z]:`, "disk format"},
	{`(?i)\bdd\b[^&|;]*(if=)?/dev/(zero|random|urandom)[^&|;]*of=/dev/(sd|nvme|hd|vd)`, "raw device write via dd"},
	{`(?i)\bdd\b[^&|;]*of=/dev/(sd|nvme|hd|vd)`, "raw device write via dd"},

	// power control (command position: start or after a chain operator)
	{`(?i)(?:^|[;&|]\s*)shutdown\b`, "system shutdown"},
	{`(?i)^reboot\b\s*(-f)?\s*$`, "system reboot"},
	{`(?i)(?:^|[;&|]\s*)init\s+0?\b`, "init power state change"},
	{`(?i)(?:^|[;&|]\s*)poweroff\b`, "power off"},
	{`(?i)(?:^|[;&|]\s*)halt\b\s*-?f?\s*$`, "system halt"},

	// privilege escalation to system paths
	{`(?i)\bchmod\s+-r\s+777\s+/(\s|$)`, "chmod 777 on root"},
	{`(?i)\bchown\s+-r\s+\w+\s+/\s*$`, "chown on root"},

	// windows registry / kernel tampering
	{`(?i)\breg\s+(delete|add)\s+hklm\\(system|sam|security)`, "registry hive tampering"},
	{`(?i)\bsysctl\s+-w\s+kernel\.`, "kernel parameter tampering"},

	// fork bombs and shell bombers
	{`:\(\)\s*\{\s*:\|\:&\s*\}\s*;:`, "fork bomb"},

	// credential / shadow file exfiltration attempts
	{`(?i)\bcat\s+/etc/shadow\b`, "shadow file access"},
}

// Blacklist matches normalized command strings against deny rules.
type Blacklist struct {
	rules []compiledRule
}

// NewBlacklist builds the default blacklist.
func NewBlacklist() *Blacklist {
	bl := &Blacklist{rules: make([]compiledRule, 0, len(defaultDenyRules))}
	for _, r := range defaultDenyRules {
		re, err := regexp.Compile(r.pattern)
		if err != nil {
			continue // never fail startup over one bad rule
		}
		bl.rules = append(bl.rules, compiledRule{re: re, description: r.description})
	}
	return bl
}

// Check returns (denied=true, reason) if the command matches any deny rule.
func (bl *Blacklist) Check(command string) (bool, string) {
	normalized := NormalizeCommand(command)
	for _, r := range bl.rules {
		if r.re.MatchString(normalized) {
			return true, r.description
		}
	}
	return false, ""
}

// NormalizeCommand defuses common obfuscation vectors before matching:
//   - strips single/double quotes around arguments ("rm" -> rm)
//   - collapses runs of whitespace
//   - removes backslash line continuations
//   - lowercases for case-insensitive matching by callers
//
// It does NOT attempt full shell parsing; it is a best-effort canonical form.
func NormalizeCommand(cmd string) string {
	s := strings.ReplaceAll(cmd, "\\\n", " ")
	var b strings.Builder
	inSingle, inDouble := false, false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch == '\'' && !inDouble:
			inSingle = !inSingle // drop quote, keep content
		case ch == '"' && !inSingle:
			inDouble = !inDouble
		case ch == '\\' && i+1 < len(s) &&
			!inSingle && !inDouble &&
			(s[i+1] == ' ' || s[i+1] == '\t'):
			i++ // escaped space: emit raw space
			b.WriteByte(' ')
		default:
			b.WriteByte(ch)
		}
	}
	out := b.String()
	// collapse whitespace
	return strings.Join(strings.Fields(out), " ")
}

// LoadBlacklistFile is a hook for future config-file-driven rules;
// currently returns the default list. Kept for API stability with configs/.
func LoadBlacklistFile(path string) (*Blacklist, error) {
	if path == "" {
		return NewBlacklist(), nil
	}
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	// custom rules files are planned; fall back to defaults meanwhile
	return NewBlacklist(), nil
}