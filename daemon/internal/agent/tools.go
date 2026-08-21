package agent

// Tool defines the schema LLM can invoke.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
	Level       int            `json:"level"` // 0 read, 1 write, 2 system
}

var DefaultTools = []Tool{
	{Name: "exec_shell", Description: "Execute shell command", Parameters: map[string]any{"command": map[string]string{"type": "string"}}, Level: 1},
	{Name: "read_file", Description: "Read file from workspace", Parameters: map[string]any{"path": map[string]string{"type": "string"}}, Level: 0},
	{Name: "search_index", Description: "Search workspace index", Parameters: map[string]any{"query": map[string]string{"type": "string"}}, Level: 0},
}
