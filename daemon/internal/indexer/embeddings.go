// Package indexer — embeddings.go
//
// Vector embedding pipeline backed by the local Ollama API on 127.0.0.1:11434.
// Batches chunks, retries transient failures, and degrades gracefully when the
// Ollama server is unreachable (indexing continues without vectors; similarity
// search then falls back to lexical matching).
package indexer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	defaultOllamaURL   = "http://127.0.0.1:11434"
	defaultEmbedModel  = "nomic-embed-text"
	embedBatchSize     = 16
	embedMaxRetries    = 3
	embedRetryBackoff  = 500 * time.Millisecond
	httpClientTimeout  = 30 * time.Second
	maxEmbedInputChars = 8000 // Ollama context guard per chunk
)

// Embedder produces vector embeddings for text chunks.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Dim() int
	Available(ctx context.Context) bool
}

// OllamaEmbedder talks to the local Ollama /api/embed endpoint.
type OllamaEmbedder struct {
	baseURL string
	model   string
	client  *http.Client

	mu      sync.Mutex
	lastDim int
}

// NewOllamaEmbedder creates an embedder; empty url/model selects defaults.
func NewOllamaEmbedder(baseURL, model string) *OllamaEmbedder {
	if baseURL == "" {
		baseURL = defaultOllamaURL
	}
	baseURL = strings.TrimRight(baseURL, "/")
	if model == "" {
		model = defaultEmbedModel
	}
	return &OllamaEmbedder{
		baseURL: baseURL,
		model:   model,
		client:  &http.Client{Timeout: httpClientTimeout},
	}
}

// OllamaEmbedRequest is the /api/embed payload.
type OllamaEmbedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// OllamaEmbedResponse is the /api/embed result.
type OllamaEmbedResponse struct {
	Model     string      `json:"model"`
	Embeddings [][]float32 `json:"embeddings"`
}

// Available reports whether the Ollama server answers.
func (o *OllamaEmbedder) Available(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.baseURL+"/api/version", nil)
	if err != nil {
		return false
	}
	resp, err := o.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// Dim returns the last observed embedding dimensionality (0 if unknown).
func (o *OllamaEmbedder) Dim() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.lastDim
}

// Embed computes embeddings for all texts in batches with retries.
// len(result) always equals len(texts); failed batches yield nil rows so
// callers can skip them without losing positional alignment.
func (o *OllamaEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for start := 0; start < len(texts); start += embedBatchSize {
		end := min(start+embedBatchSize, len(texts))

		batch := make([]string, 0, end-start)
		for _, t := range texts[start:end] {
			batch = append(batch, clampText(t))
		}

		vecs, err := o.embedBatch(ctx, batch)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			// keep alignment: leave nils for this batch, continue others
			continue
		}
		copy(out[start:end], vecs)
	}
	return out, nil
}

func (o *OllamaEmbedder) embedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	payload, err := json.Marshal(OllamaEmbedRequest{Model: o.model, Input: texts})
	if err != nil {
		return nil, fmt.Errorf("marshal embed request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt < embedMaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(embedRetryBackoff * time.Duration(attempt)):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/api/embed", bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := o.client.Do(req)
		if err != nil {
			lastErr = err
			continue // transport error -> retry
		}

		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("ollama embed status %d: %s", resp.StatusCode, truncate(string(body), 200))
			continue
		}
		if readErr != nil {
			lastErr = fmt.Errorf("read embed response: %w", readErr)
			continue
		}

		var parsed OllamaEmbedResponse
		if err := json.Unmarshal(body, &parsed); err != nil {
			lastErr = fmt.Errorf("parse embed response: %w", err)
			continue
		}
		if len(parsed.Embeddings) != len(texts) {
			lastErr = fmt.Errorf("embedding count mismatch: got %d want %d", len(parsed.Embeddings), len(texts))
			continue
		}
		o.mu.Lock()
		if len(parsed.Embeddings) > 0 {
			o.lastDim = len(parsed.Embeddings[0])
		}
		o.mu.Unlock()
		return parsed.Embeddings, nil
	}
	return nil, fmt.Errorf("embed after %d attempts: %w", embedMaxRetries, lastErr)
}

// clampText truncates oversized inputs and strips NUL bytes.
func clampText(s string) string {
	s = strings.ReplaceAll(s, "\x00", "")
	if len(s) > maxEmbedInputChars {
		return s[:maxEmbedInputChars]
	}
	return s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// CosineSimilarity computes cosine(a,b). Returns 0 for mismatched dims or
// zero vectors.
func CosineSimilarity(a, b []float32) (float64, error) {
	if len(a) == 0 || len(a) != len(b) {
		return 0, errors.New("cosine: dimension mismatch")
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0, nil
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb)), nil
}