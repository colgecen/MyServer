package guardrail

import (
	"os"
	"strings"
)

// envAllowPrefixes are the variable families a child process normally needs.
var envAllowPrefixes = []string{
	"PATH=", "HOME=", "USER=", "LOGNAME=", "SHELL=", "TERM=",
	"LANG=", "LC_", "TZ=", "TMPDIR=", "XDG_",
}

// envDenyExact are variables that alter interpreter/loader behaviour and are
// classic injection vectors; they are stripped even if inherited.
var envDenyExact = []string{
	"LD_PRELOAD", "LD_LIBRARY_PATH", "LD_AUDIT",
	"DYLD_INSERT_LIBRARIES", "DYLD_LIBRARY_PATH",
	"BASH_ENV", "BASH_FUNC_", "ENV", "CDPATH", "IFS",
	"GLOBIGNORE", "PROMPT_COMMAND",
	"PYTHONSTARTUP", "PYTHONPATH", "PYTHONHOME",
	"PERL5OPT", "RUBYOPT", "NODE_OPTIONS", "NODE_PATH",
	"RUSTFLAGS", "GOFLAGS", "GOPROXY",
	"SHELLOPTS", "BASHOPTS", "PS4", "COMP_WORDBREAKS",
}

// SanitizeEnv builds the environment for child processes: only allow-listed
// inherited vars plus caller extras, with deny-listed names always dropped.
// Extra entries containing NUL or '=' -prefixed oddities are rejected.
func SanitizeEnv(extra []string) []string {
	inherited := os.Environ()
	out := make([]string, 0, len(inherited)+len(extra))
	seen := make(map[string]bool, len(inherited)+len(extra))

	add := func(kv string) {
		if kv == "" || strings.ContainsAny(kv, "\x00") {
			return
		}
		name := kv
		if i := strings.IndexByte(kv, '='); i >= 0 {
			name = kv[:i]
		} else {
			return // malformed entry without '='
		}
		if isDeniedEnv(name) {
			return
		}
		key := strings.ToUpper(name)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, kv)
	}

	for _, kv := range inherited {
		name := kv
		if i := strings.IndexByte(kv, '='); i >= 0 {
			name = kv[:i]
		}
		if !envAllowed(name) {
			continue
		}
		add(kv)
	}
	for _, kv := range extra {
		add(kv)
	}
	return out
}

func envAllowed(name string) bool {
	for _, prefix := range envAllowPrefixes {
		if strings.HasPrefix(name, strings.TrimSuffix(prefix, "=")) ||
			strings.HasPrefix(name+"=", prefix) {
			return true
		}
	}
	return false
}

func isDeniedEnv(name string) bool {
	upper := strings.ToUpper(name)
	for _, d := range envDenyExact {
		if upper == d {
			return true
		}
		// prefix families (BASH_FUNC_*, LC_* handled by allow list)
		if strings.HasSuffix(d, "_") && strings.HasPrefix(upper, d) {
			return true
		}
	}
	return false
}