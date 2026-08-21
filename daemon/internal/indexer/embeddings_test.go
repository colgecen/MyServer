package indexer_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anomalyco/myserver/daemon/internal/indexer"
)

// fakeOllama spins an httptest server mimicking POST /api/embed.
func fakeOllama(t *testing.T, failFirst int, dim int) (*httptest.Server, *int) {
	t.Helper()
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/api/embed" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var req indexer.OllamaEmbedRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if calls <= failFirst {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		vec := make([]float32, dim)
		for i := range vec {
			vec[i] = 0.1
		}
		resp := make([][]float32, len(req.Input))
		for i := range resp {
			resp[i] = vec
		}
		_ = json.NewEncoder(w).Encode(indexer.OllamaEmbedResponse{Embeddings: resp})
	}))
	return srv, &calls
}

func TestOllamaEmbedder_BatchesAndRetries(t *testing.T) {
	srv, calls := fakeOllama(t, 1, 4) // first call fails, retry succeeds
	defer srv.Close()

	emb := indexer.NewOllamaEmbedder(srv.URL, "test-model")
	texts := make([]string, 20) // > batch size 16 -> 2 batches
	for i := range texts {
		texts[i] = strings.Repeat("x", 10)
	}
	vecs, err := emb.Embed(context.Background(), texts)
	if err != nil {
		t.Fatal(err)
	}
	if len(vecs) != 20 {
		t.Fatalf("len=%d want 20", len(vecs))
	}
	// batch 1 fails once then recovers via retry; batch 2 succeeds directly
	successful := 0
	for _, v := range vecs {
		if v != nil {
			successful++
		}
	}
	if successful != 20 {
		t.Fatalf("successful=%d want 20 (retries should recover)", successful)
	}
	if *calls < 3 { // batch1: 2 attempts (fail+retry) + batch2: 1
		t.Fatalf("expected retry attempts, calls=%d", *calls)
	}
	if emb.Dim() != 4 {
		t.Fatalf("dim=%d want 4", emb.Dim())
	}
}

func TestOllamaEmbedder_AllBatchesSucceed(t *testing.T) {
	srv, _ := fakeOllama(t, 0, 8)
	defer srv.Close()

	emb := indexer.NewOllamaEmbedder(srv.URL, "test-model")
	vecs, err := emb.Embed(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	for i, v := range vecs {
		if v == nil || len(v) != 8 {
			t.Fatalf("vec[%d] invalid: %v", i, v)
		}
	}
}

func TestCosineSimilarity(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{1, 0, 0}
	c := []float32{0, 1, 0}

	same, err := indexer.CosineSimilarity(a, b)
	if err != nil || same < 0.9999 {
		t.Fatalf("same=%v err=%v", same, err)
	}
	orth, err := indexer.CosineSimilarity(a, c)
	if err != nil || orth > 0.0001 {
		t.Fatalf("orth=%v err=%v", orth, err)
	}
	if _, err := indexer.CosineSimilarity(a, []float32{1, 2}); err == nil {
		t.Fatal("expected mismatch error")
	}
}