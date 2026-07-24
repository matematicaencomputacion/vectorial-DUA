package vector

import (
	"fmt"
	"sync"
	"time"
)

// Node is an in-memory pedagogical DUA resource in vector space.
type Node struct {
	ID           string
	DimensionDUA string
	Difficulty   string
	Format       string
	ResourceURL  string
	Embedding    []float32
}

// Match is the nearest-neighbor result for a query embedding.
type Match struct {
	Node       Node
	Similarity float32
	Found      bool
}

// Index is a concurrent in-memory k-NN store with ULID uniqueness checks.
type Index struct {
	mu    sync.RWMutex
	nodes map[string]Node // keyed by full node_id
	ring  map[string]struct{} // ULID segment uniqueness ring
	order []string
}

// NewIndex creates an empty vector index.
func NewIndex() *Index {
	return &Index{
		nodes: make(map[string]Node),
		ring:  make(map[string]struct{}),
	}
}

// Upsert registers or replaces a node. Enforces unique ULID segments.
func (idx *Index) Upsert(node Node) error {
	if !ValidateNodeID(node.ID) {
		return fmt.Errorf("invalid node id: %s", node.ID)
	}
	parts, err := ParseNodeID(node.ID)
	if err != nil {
		return err
	}
	if len(node.Embedding) == 0 {
		return fmt.Errorf("embedding required for node %s", node.ID)
	}

	idx.mu.Lock()
	defer idx.mu.Unlock()

	if existing, ok := idx.nodes[node.ID]; ok {
		// Same full ID: replace embedding/metadata.
		idx.nodes[node.ID] = node
		_ = existing
		return nil
	}
	if _, taken := idx.ring[parts.ULID]; taken {
		return fmt.Errorf("ulid already present in hashing ring: %s", parts.ULID)
	}

	idx.nodes[node.ID] = node
	idx.ring[parts.ULID] = struct{}{}
	idx.order = append(idx.order, node.ID)
	return nil
}

// Len returns the number of indexed nodes.
func (idx *Index) Len() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.nodes)
}

// HasULID reports whether a ULID segment is already in the ring.
func (idx *Index) HasULID(ulid string) bool {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	_, ok := idx.ring[ulid]
	return ok
}

// Nearest finds the closest node by cosine similarity (k=1 brute-force k-NN).
func (idx *Index) Nearest(query []float32) Match {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	best := Match{Found: false, Similarity: -1}
	for _, id := range idx.order {
		node := idx.nodes[id]
		sim := CosineSimilarity(query, node.Embedding)
		if !best.Found || sim > best.Similarity {
			best = Match{Node: node, Similarity: sim, Found: true}
		}
	}
	return best
}

// RegisterNode creates a hierarchical ULID id and upserts the node.
func (idx *Index) RegisterNode(dimensionDUA, difficulty, format, resourceURL string, embedding []float32) (Node, error) {
	id, err := NewNodeID(dimensionDUA, difficulty, format)
	if err != nil {
		return Node{}, err
	}
	node := Node{
		ID:           id,
		DimensionDUA: dimensionDUA,
		Difficulty:   difficulty,
		Format:       format,
		ResourceURL:  resourceURL,
		Embedding:    append([]float32(nil), embedding...),
	}
	if err := idx.Upsert(node); err != nil {
		return Node{}, err
	}
	return node, nil
}

// SeedDemoNodes loads a small static curriculum for demos/tests.
func SeedDemoNodes(idx *Index) error {
	seeds := []struct {
		dim, diff, format, url string
		emb                    []float32
	}{
		{"Representacion", "basico", "visual", "master://nodes/env-diagram", []float32{0.92, 0.10, 0.05, 0.20, 0.15}},
		{"Accion", "basico", "practica", "ide://cells/env-exercise", []float32{0.88, 0.25, 0.10, 0.30, 0.40}},
		{"Compromiso", "basico", "conceptual", "agent://analogies/env-story", []float32{0.70, 0.55, 0.35, 0.25, 0.60}},
		{"Representacion", "basico", "conceptual", "master://nodes/parameter-card", []float32{0.15, 0.90, 0.10, 0.20, 0.25}},
		{"Accion", "basico", "practica", "ide://cells/string-quotes", []float32{0.20, 0.15, 0.85, 0.30, 0.20}},
	}
	for _, s := range seeds {
		if _, err := idx.RegisterNode(s.dim, s.diff, s.format, s.url, s.emb); err != nil {
			return err
		}
		// Ensure chronological uniqueness under high-speed registration.
		time.Sleep(time.Millisecond)
	}
	return nil
}
