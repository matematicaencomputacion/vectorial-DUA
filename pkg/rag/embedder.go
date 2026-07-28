package rag

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/vectorial-dua/avlp/pkg/vector"
)

// DefaultEmbedDims is the content-embedding dimensionality for the offline
// HashEmbedder (vector.ContentEmbedDims). Remote HTTP embedders report their
// own Dims(); the routing index must be built with NewIndexWithDims for the
// active embedder — never silent truncate/pad between spaces.
//
// Distinct from learner V_e (5 axes: Dominio, Sensorial, Frustracion, Ritmo, Autonomia).
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

// DefaultEmbedder returns HTTPEmbedder when AVLP_EMBEDDING_URL is set; otherwise
// the offline HashEmbedder at ContentEmbedDims. There is no silent fallback from
// HTTP to hash — unset the URL to run offline.
func DefaultEmbedder() Embedder {
	httpEmb, err := NewHTTPEmbedderFromEnv()
	if err != nil {
		panic(fmt.Sprintf("AVLP_EMBEDDING_URL misconfigured: %v", err))
	}
	if httpEmb != nil {
		return httpEmb
	}
	return NewHashEmbedder(DefaultEmbedDims)
}

func envOr(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}
