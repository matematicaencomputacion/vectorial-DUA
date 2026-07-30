package rag

import (
	"sort"
	"sync"

	"github.com/vectorial-dua/avlp/pkg/vector"
)

// ScoredChunk is a retrieval hit with cosine similarity.
type ScoredChunk struct {
	Chunk      Chunk
	Similarity float32
}

// Store is an in-memory chunk index using cosine similarity.
type Store struct {
	mu     sync.RWMutex
	chunks map[string]Chunk
	order  []string
}

// NewStore creates an empty chunk store.
func NewStore() *Store {
	return &Store{chunks: make(map[string]Chunk)}
}

// Upsert inserts or replaces a chunk (must include embedding).
func (s *Store) Upsert(c Chunk) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.chunks[c.ID]; !ok {
		s.order = append(s.order, c.ID)
	}
	s.chunks[c.ID] = c
}

// Len returns indexed chunk count.
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.chunks)
}

// Chunks returns a deep snapshot in insertion order.
func (s *Store) Chunks() []Chunk {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Chunk, 0, len(s.order))
	for _, id := range s.order {
		chunk := s.chunks[id]
		chunk.Embedding = append([]float32(nil), chunk.Embedding...)
		out = append(out, chunk)
	}
	return out
}

// Retrieve returns the top-k chunks by cosine similarity to query.
func (s *Store) Retrieve(query []float32, k int) []ScoredChunk {
	if k <= 0 {
		k = 3
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	hits := make([]ScoredChunk, 0, len(s.order))
	for _, id := range s.order {
		c := s.chunks[id]
		sim := vector.CosineSimilarity(query, c.Embedding)
		hits = append(hits, ScoredChunk{Chunk: c, Similarity: sim})
	}
	sort.Slice(hits, func(i, j int) bool {
		return hits[i].Similarity > hits[j].Similarity
	})
	if len(hits) > k {
		hits = hits[:k]
	}
	return hits
}
