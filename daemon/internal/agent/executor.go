package agent

import (
	"context"

	"github.com/anomalyco/myserver/daemon/internal/guardrail"
	"github.com/anomalyco/myserver/daemon/internal/llm"
)

// PlanStep is one tool invocation in a plan.
type PlanStep struct {
	Tool string `json:"tool"`
	Args string `json:"args"`
}

// Executor runs a multi-step plan with guardrail checks.
type Executor struct {
	registry *Registry
	guard    *guardrail.SafeExec
}

func NewExecutor(r *Registry, g *guardrail.SafeExec) *Executor {
	return &Executor{registry: r, guard: g}
}

func (e *Executor) ExecutePlan(ctx context.Context, steps []PlanStep) []string {
	var results []string
	for _, s := range steps {
		if !e.registry.Allowed(s.Tool, 2) {
			results = append(results, "denied: "+s.Tool)
			continue
		}
		if s.Tool == "exec_shell" {
			out := e.guard.Evaluate(guardrail.Request{ID: s.Tool, ClientID: "agent", Command: s.Args})
			if out.Decision == guardrail.DecisionDeny {
				results = append(results, "guardrail deny: "+out.Message)
				continue
			}
			if out.Decision == guardrail.DecisionConfirm {
				results = append(results, "needs confirm: "+s.Args)
				continue
			}
		}
		// simulate tool execution
		results = append(results, "executed: "+s.Tool+" "+s.Args)
		_ = llm.Parse // keep import
		_ = ctx
	}
	return results
}
