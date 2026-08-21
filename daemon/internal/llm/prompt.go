package llm

import (
	"fmt"
	"os"
	"strings"
)

// MasterPrompt loads system_prompt.txt and injects workspace context.
var masterPromptCache string

func LoadMasterPrompt(path string) (string, error) {
	if masterPromptCache != "" {
		return masterPromptCache, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("load master prompt: %w", err)
	}
	masterPromptCache = string(b)
	return masterPromptCache, nil
}

// InjectContext builds the final system message with workspace snippets.
func InjectContext(master, workspace string, snippets []string) []ChatMessage {
	var sb strings.Builder
	sb.WriteString(master)
	sb.WriteString("\n\n## WORKSPACE: ")
	sb.WriteString(workspace)
	sb.WriteString("\n")
	for i, s := range snippets {
		fmt.Fprintf(&sb, "\n### Context %d\n%s\n", i+1, s)
	}
	return []ChatMessage{
		{Role: "system", Content: sb.String()},
	}
}
