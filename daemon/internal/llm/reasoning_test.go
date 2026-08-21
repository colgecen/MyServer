package llm_test

import (
	"testing"

	"github.com/anomalyco/myserver/daemon/internal/llm"
)

func TestParse(t *testing.T) {
	raw := "<thinking>plan</thinking> Hello ```exec_shell\ngo test ./...``` final"
	p := llm.Parse(raw)
	if p.Thinking != "plan" || len(p.ToolCalls) != 1 || p.Final != "Hello  final" && p.Final != "Hello final" {
		t.Fatalf("parse %#v", p)
	}
}
