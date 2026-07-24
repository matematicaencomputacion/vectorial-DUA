package rag

import (
	"context"
	"fmt"
)

// Retriever wraps a Store + Embedder for text or vector queries.
type Retriever struct {
	Store    *Store
	Embedder Embedder
	TopK     int
}

// NewRetriever builds a retriever with defaults.
func NewRetriever(store *Store, emb Embedder, topK int) *Retriever {
	if emb == nil {
		emb = DefaultEmbedder()
	}
	if topK <= 0 {
		topK = 3
	}
	return &Retriever{Store: store, Embedder: emb, TopK: topK}
}

// RetrieveEmbedding runs top-k over a precomputed query vector.
func (r *Retriever) RetrieveEmbedding(query []float32) []ScoredChunk {
	return r.Store.Retrieve(query, r.TopK)
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
