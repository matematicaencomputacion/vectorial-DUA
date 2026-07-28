package rag

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/vectorial-dua/avlp/pkg/vector"
)

// DefaultEmbedDims is the content-embedding dimensionality shared with the
// routing index (vector.ContentEmbedDims). Distinct from learner V_e (5 axes).
const DefaultEmbedDims = vector.ContentEmbedDims

// Embedder produces fixed-dimension embeddings for text.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	Dims() int
}

// HashEmbedder is a deterministic local stub (no external API).
type HashEmbedder struct {
	dims int
}

// NewHashEmbedder returns a HashEmbedder with the given dimensionality.
func NewHashEmbedder(dims int) *HashEmbedder {
	if dims <= 0 {
		dims = DefaultEmbedDims
	}
	return &HashEmbedder{dims: dims}
}

func (h *HashEmbedder) Dims() int { return h.dims }

// Embed maps tokens into a unit-normalized bag-of-hashes vector.
func (h *HashEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	_ = ctx
	vec := make([]float32, h.dims)
	tokens := Tokenize(text)
	if len(tokens) == 0 {
		tokens = []string{"empty"}
	}
	for _, tok := range tokens {
		v := fnv64(tok)
		i := int(v % uint64(h.dims))
		sign := float32(1)
		if v&1 == 1 {
			sign = -1
		}
		vec[i] += sign
		j := int((v >> 17) % uint64(h.dims))
		vec[j] += 0.5 * sign
	}
	normalize(vec)
	return vec, nil
}

func fnv64(s string) uint64 {
	const (
		offset = 14695981039346656037
		prime  = 1099511628211
	)
	hash := uint64(offset)
	for i := 0; i < len(s); i++ {
		hash ^= uint64(s[i])
		hash *= prime
	}
	return hash
}

func normalize(v []float32) {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	if sum == 0 {
		return
	}
	inv := 1.0 / math.Sqrt(sum)
	for i := range v {
		v[i] = float32(float64(v[i]) * inv)
	}
}

// HTTPEmbedder documents the remote embedding extension point.
type HTTPEmbedder struct {
	URL    string
	APIKey string
	Model  string
	dims   int
}

// NewHTTPEmbedderFromEnv returns an HTTP embedder if AVLP_EMBEDDING_URL is set.
func NewHTTPEmbedderFromEnv(dims int) *HTTPEmbedder {
	url := strings.TrimSpace(os.Getenv("AVLP_EMBEDDING_URL"))
	if url == "" {
		return nil
	}
	if dims <= 0 {
		dims = DefaultEmbedDims
	}
	return &HTTPEmbedder{
		URL:    url,
		APIKey: os.Getenv("AVLP_EMBEDDING_API_KEY"),
		Model:  envOr("AVLP_EMBEDDING_MODEL", "text-embedding-3-small"),
		dims:   dims,
	}
}

func (h *HTTPEmbedder) Dims() int { return h.dims }

// Embed returns a clear MVP error; production should wire an HTTP client here.
func (h *HTTPEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	_ = ctx
	_ = text
	return nil, fmt.Errorf("HTTPEmbedder not wired in MVP; configure HashEmbedder or implement client for %s", h.URL)
}

// DefaultEmbedder returns the offline HashEmbedder (HTTP is opt-in later).
func DefaultEmbedder() Embedder {
	return NewHashEmbedder(DefaultEmbedDims)
}

func envOr(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}
