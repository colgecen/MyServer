package llm_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anomalyco/myserver/daemon/internal/llm"
)

func TestList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"models": []any{map[string]any{"name": "llama3.1:8b"}}})
	}))
	defer srv.Close()
	c := llm.New(srv.URL)
	models, err := c.List(context.Background())
	if err != nil || len(models) != 1 {
		t.Fatalf("list %v %v", models, err)
	}
}
