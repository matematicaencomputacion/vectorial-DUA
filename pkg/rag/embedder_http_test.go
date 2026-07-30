package rag_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/vectorial-dua/avlp/internal/testenv"
	"github.com/vectorial-dua/avlp/pkg/rag"
)

func openAIResponse(dims int) []byte {
	emb := make([]float64, dims)
	for i := range emb {
		emb[i] = float64(i+1) / float64(dims)
	}
	body, _ := json.Marshal(map[string]any{
		"data": []map[string]any{{"embedding": emb, "index": 0}},
	})
	return body
}

func TestHTTPEmbedderSuccess(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		var req struct {
			Model string   `json:"model"`
			Input []string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req.Model != "test-model" || len(req.Input) != 1 || req.Input[0] != "hola mundo" {
			t.Fatalf("unexpected request: %+v", req)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(openAIResponse(8))
	}))
	defer srv.Close()

	emb, err := rag.NewHTTPEmbedder(rag.HTTPEmbedderConfig{
		URL:    srv.URL + "/v1",
		APIKey: "secret-key",
		Model:  "test-model",
	})
	if err != nil {
		t.Fatal(err)
	}

	vec, err := emb.Embed(context.Background(), "  HÓÓLA MUNDO  ")
	if err != nil {
		t.Fatal(err)
	}
	if len(vec) != 8 {
		t.Fatalf("len=%d want 8", len(vec))
	}
	if emb.Dims() != 8 {
		t.Fatalf("Dims()=%d want 8", emb.Dims())
	}
	if gotAuth != "Bearer secret-key" {
		t.Fatalf("auth=%q", gotAuth)
	}
}

func TestHTTPEmbedder401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
	}))
	defer srv.Close()

	emb, err := rag.NewHTTPEmbedder(rag.HTTPEmbedderConfig{URL: srv.URL, Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = emb.Embed(context.Background(), "x")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "401") || !strings.Contains(err.Error(), "AVLP_EMBEDDING_API_KEY") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHTTPEmbedderInconsistentDimsBetweenCalls(t *testing.T) {
	call := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call++
		dims := 4
		if call > 1 {
			dims = 6
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(openAIResponse(dims))
	}))
	defer srv.Close()

	emb, err := rag.NewHTTPEmbedder(rag.HTTPEmbedderConfig{URL: srv.URL, Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := emb.Embed(context.Background(), "first"); err != nil {
		t.Fatal(err)
	}
	_, err = emb.Embed(context.Background(), "second")
	if err == nil {
		t.Fatal("expected inconsistent dims error")
	}
	if !strings.Contains(err.Error(), "inconsistent embedding dims") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHTTPEmbedderTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(openAIResponse(4))
	}))
	defer srv.Close()

	emb, err := rag.NewHTTPEmbedder(rag.HTTPEmbedderConfig{
		URL:     srv.URL,
		Model:   "m",
		Timeout: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = emb.Embed(context.Background(), "slow")
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "request failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHTTPEmbedderRetryOn5xx(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(openAIResponse(3))
	}))
	defer srv.Close()

	emb, err := rag.NewHTTPEmbedder(rag.HTTPEmbedderConfig{URL: srv.URL, Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	vec, err := emb.Embed(context.Background(), "retry-me")
	if err != nil {
		t.Fatal(err)
	}
	if len(vec) != 3 {
		t.Fatalf("len=%d", len(vec))
	}
	if attempts != 2 {
		t.Fatalf("attempts=%d want 2", attempts)
	}
}

func TestDefaultEmbedderHashOffline(t *testing.T) {
	t.Setenv("AVLP_EMBEDDING_URL", "")
	emb := rag.DefaultEmbedder()
	if _, ok := emb.(*rag.HashEmbedder); !ok {
		t.Fatalf("expected HashEmbedder, got %T", emb)
	}
	if emb.Dims() != rag.DefaultEmbedDims {
		t.Fatalf("dims=%d", emb.Dims())
	}
}

func TestDefaultEmbedderEHTTPFromEnv(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(openAIResponse(5))
	}))
	defer srv.Close()

	t.Setenv("AVLP_EMBEDDING_URL", srv.URL)
	t.Setenv("AVLP_EMBEDDING_MODEL", "env-model")

	emb, err := rag.DefaultEmbedderE()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := emb.(*rag.HTTPEmbedder); !ok {
		t.Fatalf("expected HTTPEmbedder, got %T", emb)
	}
	if err := rag.EnsureEmbedderDims(context.Background(), emb); err != nil {
		t.Fatal(err)
	}
	if emb.Dims() != 5 {
		t.Fatalf("dims=%d want 5", emb.Dims())
	}
}

func TestDefaultEmbedderEMisconfiguredDims(t *testing.T) {
	t.Setenv("AVLP_EMBEDDING_URL", "http://example.com/v1")
	t.Setenv("AVLP_EMBEDDING_DIMS", "not-a-number")
	if _, err := rag.DefaultEmbedderE(); err == nil {
		t.Fatal("expected error for invalid AVLP_EMBEDDING_DIMS")
	}
}

func TestEnsureEmbedderDimsExplicitHint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not call server when dims hint is set")
	}))
	defer srv.Close()

	emb, err := rag.NewHTTPEmbedder(rag.HTTPEmbedderConfig{
		URL:      srv.URL,
		Model:    "m",
		DimsHint: 12,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := rag.EnsureEmbedderDims(context.Background(), emb); err != nil {
		t.Fatal(err)
	}
	if emb.Dims() != 12 {
		t.Fatalf("dims=%d", emb.Dims())
	}
}

func TestNewHTTPEmbedderFromEnvInvalidDims(t *testing.T) {
	t.Setenv("AVLP_EMBEDDING_URL", "http://example.com/v1")
	t.Setenv("AVLP_EMBEDDING_DIMS", "not-a-number")
	if _, err := rag.NewHTTPEmbedderFromEnv(); err == nil {
		t.Fatal("expected error for invalid AVLP_EMBEDDING_DIMS")
	}
}

func TestNormalizeEmbeddingURLPreservesFullPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/custom/embeddings" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(openAIResponse(2))
	}))
	defer srv.Close()

	emb, err := rag.NewHTTPEmbedder(rag.HTTPEmbedderConfig{
		URL:   srv.URL + "/custom/embeddings",
		Model: "m",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := emb.Embed(context.Background(), "x"); err != nil {
		t.Fatal(err)
	}
}

func TestMain(m *testing.M) {
	// Keep tests isolated from developer shell config: embedder URL/model and
	// the RAG similarity floor all come from AVLP_*.
	testenv.Clear()
	os.Exit(m.Run())
}
