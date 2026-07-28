package rag

import (
	"context"
	"fmt"
	"os"
	"strconv"
)

// DefaultMinSimilarity is the conservative floor for hash/simmatrix against
// data/knowledge_base: on-topic PostGIS chunks sit ~0.33; off-topic “bit” ≤0.19.
// Raise AVLP_RAG_MIN_SIMILARITY when using denser embedders (e.g. bge-m3).
const DefaultMinSimilarity float32 = 0.30

// Retriever wraps a Store + Embedder for text or vector queries.
type Retriever struct {
	Store         *Store
	Embedder      Embedder
	TopK          int
	MinSimilarity float32 // hits strictly below this are discarded; <0 → use env/default
}

// MinSimilarityFromEnv reads AVLP_RAG_MIN_SIMILARITY (default DefaultMinSimilarity).
// Set to 0 to disable the floor (keep raw top-k).
func MinSimilarityFromEnv() float32 {
	v := os.Getenv("AVLP_RAG_MIN_SIMILARITY")
	if v == "" {
		return DefaultMinSimilarity
	}
	f, err := strconv.ParseFloat(v, 32)
	if err != nil || f < 0 || f > 1 {
		return DefaultMinSimilarity
	}
	return float32(f)
}

// NewRetriever builds a retriever with defaults.
// Prefer an explicit Embedder; nil falls back to the offline HashEmbedder.
// MinSimilarity is taken from AVLP_RAG_MIN_SIMILARITY (default 0.30).
func NewRetriever(store *Store, emb Embedder, topK int) *Retriever {
	if emb == nil {
		emb = NewHashEmbedder(DefaultEmbedDims)
	}
	if topK <= 0 {
		topK = 3
	}
	return &Retriever{
		Store:         store,
		Embedder:      emb,
		TopK:          topK,
		MinSimilarity: MinSimilarityFromEnv(),
	}
}

// FilterByMinSimilarity drops hits below the floor (inclusive keep when sim >= min).
// min < 0 means “use r.MinSimilarity”; when that is also unset, DefaultMinSimilarity.
func FilterByMinSimilarity(hits []ScoredChunk, min float32) []ScoredChunk {
	if min < 0 {
		min = DefaultMinSimilarity
	}
	if min == 0 {
		return hits
	}
	out := hits[:0:0]
	for _, h := range hits {
		if h.Similarity >= min {
			out = append(out, h)
		}
	}
	return out
}

// RetrieveEmbedding runs top-k over a precomputed query vector, then applies MinSimilarity.
func (r *Retriever) RetrieveEmbedding(query []float32) []ScoredChunk {
	hits := r.Store.Retrieve(query, r.TopK)
	min := r.MinSimilarity
	if min < 0 {
		min = DefaultMinSimilarity
	}
	return FilterByMinSimilarity(hits, min)
}

// RetrieveText embeds text then retrieves.
func (r *Retriever) RetrieveText(ctx context.Context, text string) ([]ScoredChunk, error) {
	if r.Store == nil || r.Store.Len() == 0 {
		return nil, fmt.Errorf("rag store is empty")
	}
	emb, err := r.Embedder.Embed(ctx, text)
	if err != nil {
		return nil, err
	}
	return r.RetrieveEmbedding(emb), nil
}

// Sources extracts unique source paths from scored chunks.
func Sources(hits []ScoredChunk) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, h := range hits {
		if _, ok := seen[h.Chunk.Source]; ok {
			continue
		}
		seen[h.Chunk.Source] = struct{}{}
		out = append(out, h.Chunk.Source)
	}
	return out
}
