package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	baseURL string
	http    *http.Client
}

func New(baseURL string) *Client {
	if baseURL == "" {
		baseURL = "http://127.0.0.1:11434"
	}
	return &Client{baseURL: baseURL, http: &http.Client{Timeout: 120 * time.Second}}
}

type Model struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

func (c *Client) List(ctx context.Context) ([]Model, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/tags", nil)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out struct {
		Models []Model `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Models, nil
}

func (c *Client) Pull(ctx context.Context, name string, onProgress func(string)) error {
	body, _ := json.Marshal(map[string]string{"name": name})
	req, _ := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/pull", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		var m map[string]any
		json.Unmarshal(sc.Bytes(), &m)
		if s, ok := m["status"].(string); ok && onProgress != nil {
			onProgress(s)
		}
	}
	return sc.Err()
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

func (c *Client) ChatStream(ctx context.Context, model string, msgs []ChatMessage, onToken func(string)) error {
	reqBody, _ := json.Marshal(ChatRequest{Model: model, Messages: msgs, Stream: true})
	req, _ := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/chat", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("ollama chat %d", resp.StatusCode)
	}
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		var chunk struct {
			Message ChatMessage `json:"message"`
			Done    bool        `json:"done"`
		}
		if json.Unmarshal(sc.Bytes(), &chunk) == nil && chunk.Message.Content != "" && onToken != nil {
			onToken(chunk.Message.Content)
		}
		if chunk.Done {
			break
		}
	}
	return sc.Err()
}

type EmbedRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}
type EmbedResponse struct {
	Embedding []float32 `json:"embedding"`
}

func (c *Client) Embed(ctx context.Context, model, prompt string) ([]float32, error) {
	body, _ := json.Marshal(EmbedRequest{Model: model, Prompt: prompt})
	req, _ := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/embeddings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out EmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Embedding, nil
}
