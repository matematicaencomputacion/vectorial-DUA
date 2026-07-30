package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultEmbeddingTimeout = 10 * time.Second

// HTTPEmbedderConfig configures a remote OpenAI-compatible embedding client.
type HTTPEmbedderConfig struct {
	URL        string
	APIKey     string
	Model      string
	DimsHint   int
	Timeout    time.Duration
	MaxRetries int
	Client     *http.Client
}

// HTTPEmbedder calls a remote OpenAI-compatible /embeddings endpoint.
type HTTPEmbedder struct {
	endpoint   string
	apiKey     string
	model      string
	dimsHint   int
	dims       int
	timeout    time.Duration
	maxRetries int
	client     *http.Client

	mu sync.Mutex
}

type embeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embeddingResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// NewHTTPEmbedder builds an HTTP embedder from explicit config (tests and wiring).
func NewHTTPEmbedder(cfg HTTPEmbedderConfig) (*HTTPEmbedder, error) {
	url := strings.TrimSpace(cfg.URL)
	if url == "" {
		return nil, fmt.Errorf("HTTPEmbedder: URL is required")
	}
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = envOr("AVLP_EMBEDDING_MODEL", "text-embedding-3-small")
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = embeddingTimeoutFromEnv()
	}
	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 1
	}
	client := cfg.Client
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	return &HTTPEmbedder{
		endpoint:   normalizeEmbeddingURL(url),
		apiKey:     cfg.APIKey,
		model:      model,
		dimsHint:   cfg.DimsHint,
		timeout:    timeout,
		maxRetries: maxRetries,
		client:     client,
	}, nil
}

// NewHTTPEmbedderFromEnv returns an HTTP embedder when AVLP_EMBEDDING_URL is set.
func NewHTTPEmbedderFromEnv() (*HTTPEmbedder, error) {
	url := strings.TrimSpace(os.Getenv("AVLP_EMBEDDING_URL"))
	if url == "" {
		return nil, nil
	}
	dimsHint := 0
	if v := strings.TrimSpace(os.Getenv("AVLP_EMBEDDING_DIMS")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("AVLP_EMBEDDING_DIMS=%q: want positive integer", v)
		}
		dimsHint = n
	}
	return NewHTTPEmbedder(HTTPEmbedderConfig{
		URL:      url,
		APIKey:   os.Getenv("AVLP_EMBEDDING_API_KEY"),
		Model:    envOr("AVLP_EMBEDDING_MODEL", "text-embedding-3-small"),
		DimsHint: dimsHint,
	})
}

func (h *HTTPEmbedder) Dims() int {
	if h == nil {
		return 0
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.dims > 0 {
		return h.dims
	}
	return h.dimsHint
}

// ProbeDims discovers embedding dimensionality when not set explicitly.
func (h *HTTPEmbedder) ProbeDims(ctx context.Context) error {
	if h == nil {
		return fmt.Errorf("HTTPEmbedder is nil")
	}
	if h.Dims() > 0 {
		return nil
	}
	_, err := h.Embed(ctx, "avlp-dim-probe")
	return err
}

// Embed requests a vector from the remote embeddings API.
func (h *HTTPEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if h == nil {
		return nil, fmt.Errorf("HTTPEmbedder is nil")
	}
	text = NormalizeForEmbed(text)
	if text == "" {
		text = " "
	}

	body, err := json.Marshal(embeddingRequest{
		Model: h.model,
		Input: []string{text},
	})
	if err != nil {
		return nil, fmt.Errorf("HTTPEmbedder: marshal request: %w", err)
	}

	var lastErr error
	attempts := h.maxRetries + 1
	for attempt := 0; attempt < attempts; attempt++ {
		vec, status, err := h.doEmbed(ctx, body)
		if err == nil {
			if err := h.adoptDims(len(vec)); err != nil {
				return nil, err
			}
			return vec, nil
		}
		lastErr = err
		if !retryableHTTP(status, err) || attempt == attempts-1 {
			break
		}
	}
	return nil, fmt.Errorf("HTTPEmbedder %s: %w", h.endpoint, lastErr)
}

func (h *HTTPEmbedder) doEmbed(ctx context.Context, body []byte) ([]float32, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if h.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+h.apiKey)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed (timeout=%s): %w", h.timeout, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, resp.StatusCode, fmt.Errorf("unauthorized (HTTP 401): check AVLP_EMBEDDING_API_KEY")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(raw))
		var parsed embeddingResponse
		if json.Unmarshal(raw, &parsed) == nil && parsed.Error != nil && parsed.Error.Message != "" {
			msg = parsed.Error.Message
		}
		return nil, resp.StatusCode, fmt.Errorf("HTTP %d: %s", resp.StatusCode, msg)
	}

	var out embeddingResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("decode response: %w", err)
	}
	if len(out.Data) == 0 || len(out.Data[0].Embedding) == 0 {
		return nil, resp.StatusCode, fmt.Errorf("empty embedding in response")
	}

	vec := make([]float32, len(out.Data[0].Embedding))
	for i, v := range out.Data[0].Embedding {
		vec[i] = float32(v)
	}
	return vec, resp.StatusCode, nil
}

func (h *HTTPEmbedder) adoptDims(n int) error {
	if n <= 0 {
		return fmt.Errorf("invalid embedding length %d", n)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.dimsHint > 0 && h.dimsHint != n {
		return fmt.Errorf("embedding dims %d contradict AVLP_EMBEDDING_DIMS=%d", n, h.dimsHint)
	}
	if h.dims == 0 {
		h.dims = n
		return nil
	}
	if h.dims != n {
		return fmt.Errorf("inconsistent embedding dims: first call %d, got %d", h.dims, n)
	}
	return nil
}

func normalizeEmbeddingURL(raw string) string {
	u := strings.TrimRight(strings.TrimSpace(raw), "/")
	if strings.HasSuffix(u, "/embeddings") {
		return u
	}
	return u + "/embeddings"
}

func embeddingTimeoutFromEnv() time.Duration {
	v := strings.TrimSpace(os.Getenv("AVLP_EMBEDDING_TIMEOUT"))
	if v == "" {
		return defaultEmbeddingTimeout
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return defaultEmbeddingTimeout
	}
	return d
}

func retryableHTTP(status int, err error) bool {
	if err != nil {
		return true
	}
	return status >= 500
}

// EnsureEmbedderDims guarantees Dims() > 0, probing HTTP embedders when needed.
func EnsureEmbedderDims(ctx context.Context, emb Embedder) error {
	if emb == nil {
		return fmt.Errorf("embedder is nil")
	}
	if emb.Dims() > 0 {
		return nil
	}
	if p, ok := emb.(*HTTPEmbedder); ok {
		return p.ProbeDims(ctx)
	}
	_, err := emb.Embed(ctx, "avlp-dim-probe")
	if err != nil {
		return fmt.Errorf("embedder dim probe: %w", err)
	}
	if emb.Dims() <= 0 {
		return fmt.Errorf("embedder still reports unknown dims after probe")
	}
	return nil
}
