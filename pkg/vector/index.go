package vector

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// Node is an in-memory pedagogical DUA resource in vector space.
type Node struct {
	ID              string
	DimensionDUA    string
	Difficulty      string
	Format          string
	ResourceURL     string
	Embedding       []float32
	IsLiveGenerated bool // RAG-generated station, not reviewed curriculum
	CreatedAt       time.Time
	Concepts        []string // knowledge concept ids or slugs (Ola 7)
}

// LivePreferenceMargin is how much better a live-generated station must score
// than the best curated node before it wins static matching.
//
// Live stations stay in the index so a student who repeats a novel doubt lands
// on the station already built for them instead of paying for a new one. But one
// accumulates per miss, and an unweighted k-NN lets them eclipse curated nodes
// after a long session — precisely the nodes that carry the botonera, the DUA
// schemas and reviewed pedagogy. The margin keeps curated material as the
// default without making live stations unreachable.
const LivePreferenceMargin = 0.05

const defaultLiveNodeTTL = 24 * time.Hour

// Match is the nearest-neighbor result for a query embedding.
type Match struct {
	Node       Node
	Similarity float32
	Found      bool
}

// Index is a concurrent in-memory k-NN store with ULID uniqueness checks.
type Index struct {
	mu    sync.Mutex
	dims  int                 // required content embedding length (ContentEmbedDims)
	nodes map[string]Node     // keyed by full node_id
	ring  map[string]struct{} // ULID segment uniqueness ring
	order []string
	ttl   time.Duration
	now   func() time.Time
}

// NewIndex creates an empty vector index locked to ContentEmbedDims.
func NewIndex() *Index {
	return NewIndexWithDims(ContentEmbedDims)
}

// NewIndexWithDims creates an index that rejects embeddings of any other length.
func NewIndexWithDims(dims int) *Index {
	if dims <= 0 {
		dims = ContentEmbedDims
	}
	return &Index{
		dims:  dims,
		nodes: make(map[string]Node),
		ring:  make(map[string]struct{}),
		ttl:   LiveNodeTTLFromEnv(),
		now:   func() time.Time { return time.Now().UTC() },
	}
}

// LiveNodeTTLFromEnv reads AVLP_LIVE_NODE_TTL (Go duration, default 24h).
func LiveNodeTTLFromEnv() time.Duration {
	v := strings.TrimSpace(os.Getenv("AVLP_LIVE_NODE_TTL"))
	if v == "" {
		return defaultLiveNodeTTL
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return defaultLiveNodeTTL
	}
	return d
}

// Dims returns the required content embedding dimensionality for this index.
func (idx *Index) Dims() int {
	if idx == nil || idx.dims <= 0 {
		return ContentEmbedDims
	}
	return idx.dims
}

// Upsert registers or replaces a node. Enforces unique ULID segments and
// embedding dimensionality matching the index.
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
	want := idx.Dims()
	if len(node.Embedding) != want {
		return fmt.Errorf("embedding dims mismatch for node %s: got %d, index requires %d (content space; not V_e)",
			node.ID, len(node.Embedding), want)
	}

	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.purgeLiveLocked(idx.now())

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
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.purgeLiveLocked(idx.now())
	return len(idx.nodes)
}

// HasULID reports whether a ULID segment is already in the ring.
func (idx *Index) HasULID(ulid string) bool {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.purgeLiveLocked(idx.now())
	_, ok := idx.ring[ulid]
	return ok
}

// Nodes returns a snapshot of all indexed nodes in insertion order.
func (idx *Index) Nodes() []Node {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.purgeLiveLocked(idx.now())
	out := make([]Node, 0, len(idx.order))
	for _, id := range idx.order {
		n := idx.nodes[id]
		n.Embedding = append([]float32(nil), n.Embedding...)
		out = append(out, n)
	}
	return out
}

// Nearest finds the closest node by cosine similarity (k=1 brute-force k-NN).
//
// Curated nodes win ties and near-ties: a live-generated station only wins when
// it beats the best curated node by more than LivePreferenceMargin.
func (idx *Index) Nearest(query []float32) Match {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.purgeLiveLocked(idx.now())

	curated := Match{Found: false, Similarity: -1}
	live := Match{Found: false, Similarity: -1}
	for _, id := range idx.order {
		node := idx.nodes[id]
		best := &curated
		if node.IsLiveGenerated {
			best = &live
		}
		sim := CosineSimilarity(query, node.Embedding)
		if !best.Found || sim > best.Similarity {
			*best = Match{Node: node, Similarity: sim, Found: true}
		}
	}

	switch {
	case !live.Found:
		return curated
	case !curated.Found:
		return live
	case live.Similarity > curated.Similarity+LivePreferenceMargin:
		return live
	default:
		return curated
	}
}

// RegisterNode creates a hierarchical ULID id and upserts a curated node.
// Embeddings must match the index dimensionality (see FitIndexEmbedding).
func (idx *Index) RegisterNode(dimensionDUA, difficulty, format, resourceURL string, embedding []float32) (Node, error) {
	return idx.RegisterNodeWithConcepts(dimensionDUA, difficulty, format, resourceURL, embedding, nil)
}

// RegisterNodeWithConcepts is RegisterNode plus stable knowledge-concept links.
func (idx *Index) RegisterNodeWithConcepts(dimensionDUA, difficulty, format, resourceURL string, embedding []float32, concepts []string) (Node, error) {
	return idx.registerNode(dimensionDUA, difficulty, format, resourceURL, embedding, false, concepts)
}

// RegisterLiveNode registers a RAG-generated station, matched under
// LivePreferenceMargin so it never quietly displaces curated material.
func (idx *Index) RegisterLiveNode(dimensionDUA, difficulty, format, resourceURL string, embedding []float32) (Node, error) {
	return idx.registerNode(dimensionDUA, difficulty, format, resourceURL, embedding, true, nil)
}

func (idx *Index) registerNode(dimensionDUA, difficulty, format, resourceURL string, embedding []float32, isLive bool, concepts []string) (Node, error) {
	fitted, err := FitIndexEmbedding(embedding, idx.Dims())
	if err != nil {
		return Node{}, err
	}
	id, err := NewNodeID(dimensionDUA, difficulty, format)
	if err != nil {
		return Node{}, err
	}
	node := Node{
		ID:              id,
		DimensionDUA:    dimensionDUA,
		Difficulty:      difficulty,
		Format:          format,
		ResourceURL:     resourceURL,
		Embedding:       fitted,
		IsLiveGenerated: isLive,
		Concepts:        append([]string(nil), concepts...),
	}
	if isLive {
		node.CreatedAt = idx.now()
	}
	if err := idx.Upsert(node); err != nil {
		return Node{}, err
	}
	return node, nil
}

func (idx *Index) purgeLiveLocked(now time.Time) {
	if idx.ttl <= 0 {
		return
	}
	kept := idx.order[:0]
	for _, id := range idx.order {
		node, ok := idx.nodes[id]
		if !ok {
			continue
		}
		expired := node.IsLiveGenerated &&
			!node.CreatedAt.IsZero() &&
			now.Sub(node.CreatedAt) > idx.ttl
		if !expired {
			kept = append(kept, id)
			continue
		}
		delete(idx.nodes, id)
		if parts, err := ParseNodeID(id); err == nil {
			delete(idx.ring, parts.ULID)
		}
	}
	idx.order = kept
}

// TextEmbedder is the content embedding contract used by seed loaders.
type TextEmbedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// SeedDemoNodes loads a small static curriculum for demos/tests.
// Embeddings are generated from natural-language pedagogical descriptors so
// paraphrased student queries can match under a semantic embedder.
func SeedDemoNodes(idx *Index, emb TextEmbedder) error {
	if emb == nil {
		return fmt.Errorf("seed embedder is required")
	}
	seeds := []struct {
		dim, diff, format, url string
		text                   string
		concepts               []string
	}{
		{
			"Representacion", "basico", "visual", "master://nodes/env-diagram",
			"Diagrama visual que explica qué son las variables de entorno y el archivo .env: cómo se leen en ejecución y cómo representar la configuración separada del código. Responde preguntas como: ¿qué es el archivo .env?, ¿me lo mostrás con un diagrama?, ¿cómo se ve la configuración fuera del código?",
			[]string{"concept:env-file"},
		},
		{
			"Accion", "basico", "practica", "ide://cells/env-exercise",
			"Ejercicio guiado paso a paso para crear y leer un archivo .env en el IDE: escribir la variable, cargarla en el código y verificar que el programa la lee. Responde preguntas como: ¿cómo creo el archivo .env?, ¿cómo leo una variable de entorno desde mi código?",
			[]string{"concept:env-file"},
		},
		{
			"Compromiso", "basico", "conceptual", "agent://analogies/env-story",
			"Por qué importan las variables de entorno y cuidar los secretos: motivación, qué riesgos de seguridad hay al exponer credenciales, y qué te ahorra separar la configuración del código. Responde preguntas como: ¿por qué debería importarme configurar el .env?, ¿qué riesgo hay si subo mis claves al repo?, ¿para qué separar la configuración del código?",
			[]string{"concept:env-secrets"},
		},
		{
			"Representacion", "basico", "conceptual", "master://nodes/parameter-card",
			"Tarjeta conceptual que define qué es un parámetro en programación: para qué sirve, cómo se distingue de una variable de entorno, y un ejemplo breve de uso. Responde preguntas como: ¿qué es un parámetro?, ¿qué diferencia hay entre un parámetro y una variable normal?",
			[]string{"concept:function-parameters"},
		},
		{
			"Accion", "basico", "practica", "ide://cells/string-quotes",
			"Práctica de depuración con comillas y strings: errores comunes al abrir o cerrar comillas, cómo corregirlos en una celda y verificar la salida. Responde preguntas como: ¿por qué se rompe mi string por las comillas?, ¿puedo probarlo en una celda?",
			[]string{"concept:string-literals"},
		},
	}
	for _, s := range seeds {
		vec, err := emb.Embed(context.Background(), s.text)
		if err != nil {
			return err
		}
		if _, err := idx.RegisterNodeWithConcepts(s.dim, s.diff, s.format, s.url, vec, s.concepts); err != nil {
			return err
		}
		// Ensure chronological uniqueness under high-speed registration.
		time.Sleep(time.Millisecond)
	}
	return nil
}
