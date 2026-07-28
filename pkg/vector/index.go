package vector

import (
	"context"
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
	dims  int                 // required content embedding length (ContentEmbedDims)
	nodes map[string]Node     // keyed by full node_id
	ring  map[string]struct{} // ULID segment uniqueness ring
	order []string
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
	}
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

// Nodes returns a snapshot of all indexed nodes in insertion order.
func (idx *Index) Nodes() []Node {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	out := make([]Node, 0, len(idx.order))
	for _, id := range idx.order {
		n := idx.nodes[id]
		n.Embedding = append([]float32(nil), n.Embedding...)
		out = append(out, n)
	}
	return out
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
// Embeddings must match the index dimensionality (see FitIndexEmbedding).
func (idx *Index) RegisterNode(dimensionDUA, difficulty, format, resourceURL string, embedding []float32) (Node, error) {
	fitted, err := FitIndexEmbedding(embedding, idx.Dims())
	if err != nil {
		return Node{}, err
	}
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
		Embedding:    fitted,
	}
	if err := idx.Upsert(node); err != nil {
		return Node{}, err
	}
	return node, nil
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
	}{
		{
			"Representacion", "basico", "visual", "master://nodes/env-diagram",
			"Diagrama visual que explica qué son las variables de entorno y el archivo .env: cómo se leen en ejecución y cómo representar la configuración separada del código. Responde preguntas como: ¿qué es el archivo .env?, ¿me lo mostrás con un diagrama?, ¿cómo se ve la configuración fuera del código?",
		},
		{
			"Accion", "basico", "practica", "ide://cells/env-exercise",
			"Ejercicio guiado paso a paso para crear y leer un archivo .env en el IDE: escribir la variable, cargarla en el código y verificar que el programa la lee. Responde preguntas como: ¿cómo creo el archivo .env?, ¿cómo leo una variable de entorno desde mi código?",
		},
		{
			"Compromiso", "basico", "conceptual", "agent://analogies/env-story",
			"Por qué importan las variables de entorno y cuidar los secretos: motivación, qué riesgos de seguridad hay al exponer credenciales, y qué te ahorra separar la configuración del código. Responde preguntas como: ¿por qué debería importarme configurar el .env?, ¿qué riesgo hay si subo mis claves al repo?, ¿para qué separar la configuración del código?",
		},
		{
			"Representacion", "basico", "conceptual", "master://nodes/parameter-card",
			"Tarjeta conceptual que define qué es un parámetro en programación: para qué sirve, cómo se distingue de una variable de entorno, y un ejemplo breve de uso. Responde preguntas como: ¿qué es un parámetro?, ¿qué diferencia hay entre un parámetro y una variable normal?",
		},
		{
			"Accion", "basico", "practica", "ide://cells/string-quotes",
			"Práctica de depuración con comillas y strings: errores comunes al abrir o cerrar comillas, cómo corregirlos en una celda y verificar la salida. Responde preguntas como: ¿por qué se rompe mi string por las comillas?, ¿puedo probarlo en una celda?",
		},
	}
	for _, s := range seeds {
		vec, err := emb.Embed(context.Background(), s.text)
		if err != nil {
			return err
		}
		if _, err := idx.RegisterNode(s.dim, s.diff, s.format, s.url, vec); err != nil {
			return err
		}
		// Ensure chronological uniqueness under high-speed registration.
		time.Sleep(time.Millisecond)
	}
	return nil
}
