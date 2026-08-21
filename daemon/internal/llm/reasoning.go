package llm

import (
	"regexp"
	"strings"
)

var (
	thinkRe = regexp.MustCompile(`(?s)<thinking>(.*?)</thinking>`)
	toolRe  = regexp.MustCompile("(?s)```exec_shell\\s*(.*?)\\s*```")
)

// Parsed holds the structured output from the model.
type Parsed struct {
	Thinking   string
	ToolCalls  []string
	Final      string
	HasTool    bool
}

// Parse extracts thinking tags, exec_shell blocks and final output.
func Parse(raw string) Parsed {
	var p Parsed
	if m := thinkRe.FindStringSubmatch(raw); len(m) == 2 {
		p.Thinking = strings.TrimSpace(m[1])
		raw = thinkRe.ReplaceAllString(raw, "")
	}
	matches := toolRe.FindAllStringSubmatch(raw, -1)
	for _, m := range matches {
		p.ToolCalls = append(p.ToolCalls, strings.TrimSpace(m[1]))
	}
	p.HasTool = len(p.ToolCalls) > 0
	// final is raw without tool blocks trimmed
	p.Final = strings.TrimSpace(toolRe.ReplaceAllString(raw, ""))
	return p
}

// StreamParser incrementally accumulates tokens and emits parse updates.
type StreamParser struct {
	buf strings.Builder
}

func (s *StreamParser) Push(token string) Parsed {
	s.buf.WriteString(token)
	return Parse(s.buf.String())
}

func (s *StreamParser) Raw() string { return s.buf.String() }
