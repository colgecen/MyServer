package agent

import (
	"context"

	"github.com/anomalyco/myserver/daemon/internal/llm"
)

// Loop handles LLM tool calling roundtrips.
type Loop struct {
	client *llm.Client
	model  string
}

func NewLoop(c *llm.Client, model string) *Loop { return &Loop{client: c, model: model} }

func (l *Loop) Run(ctx context.Context, msgs []llm.ChatMessage, onToken func(string)) ([]llm.ChatMessage, error) {
	var out []llm.ChatMessage
	err := l.client.ChatStream(ctx, l.model, msgs, func(tok string) {
		out = append(out, llm.ChatMessage{Role: "assistant", Content: tok})
		if onToken != nil {
			onToken(tok)
		}
	})
	return out, err
}
