package security

import (
	"regexp"
	"strings"
)

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(api[_-]?key\s*[:=]\s*)[^\s"']+`),
	regexp.MustCompile(`(?i)(password\s*[:=]\s*)[^\s"']+`),
	regexp.MustCompile(`(?i)(token\s*[:=]\s*)[^\s"']+`),
	regexp.MustCompile(`(?i)gh[oprs]_[A-Za-z0-9_]+`),
	regexp.MustCompile(`(?i)sk-[A-Za-z0-9_-]{20,}`),
}

func Redact(s string) string {
	out := s
	for _, re := range secretPatterns {
		out = re.ReplaceAllString(out, `${1}***REDACTED***`)
	}
	// also mask long hex-like secrets
	if strings.Contains(out, "REDACTED") {
		return out
	}
	return out
}

func RedactMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		if strings.Contains(strings.ToLower(k), "key") || strings.Contains(strings.ToLower(k), "token") || strings.Contains(strings.ToLower(k), "password") {
			out[k] = "***REDACTED***"
		} else {
			out[k] = Redact(v)
		}
	}
	return out
}
