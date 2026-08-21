package agent_test

import (
	"context"
	"testing"

	"github.com/anomalyco/myserver/daemon/internal/agent"
	"github.com/anomalyco/myserver/daemon/internal/guardrail"
)

func TestExecutor_Simulation(t *testing.T) {
	reg := agent.NewRegistry()
	g := guardrail.New(nil)
	ex := agent.NewExecutor(reg, g)
	steps := []agent.PlanStep{
		{Tool: "read_file", Args: "main.go"},
		{Tool: "exec_shell", Args: "ls -la"},
		{Tool: "exec_shell", Args: "rm -rf /"},
	}
	results := ex.ExecutePlan(context.Background(), steps)
	if len(results) != 3 {
		t.Fatalf("got %v", results)
	}
	if results[2] == "executed: exec_shell rm -rf /" {
		t.Fatal("guardrail should deny rm -rf /")
	}
	// registry gating
	if !reg.Allowed("read_file", 0) {
		t.Fatal("read_file should be allowed at 0")
	}
	if reg.Allowed("exec_shell", 0) {
		t.Fatal("exec_shell should not be allowed at 0")
	}
}
