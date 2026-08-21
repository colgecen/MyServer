package agent

import (
	"fmt"
	"sync"
)

// Registry is MCP-style tool registry with permission gate.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

func NewRegistry() *Registry {
	r := &Registry{tools: make(map[string]Tool)}
	for _, t := range DefaultTools {
		r.tools[t.Name] = t
	}
	return r
}

func (r *Registry) Register(t Tool) {
	r.mu.Lock()
	r.tools[t.Name] = t
	r.mu.Unlock()
}

func (r *Registry) Get(name string) (Tool, error) {
	r.mu.RLock()
	t, ok := r.tools[name]
	r.mu.RUnlock()
	if !ok {
		return Tool{}, fmt.Errorf("tool %q not found", name)
	}
	return t, nil
}

func (r *Registry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t)
	}
	return out
}

// Allowed checks guardrail level gating.
func (r *Registry) Allowed(name string, maxLevel int) bool {
	t, err := r.Get(name)
	if err != nil {
		return false
	}
	return t.Level <= maxLevel
}
