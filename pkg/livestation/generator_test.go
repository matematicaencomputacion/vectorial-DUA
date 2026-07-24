package livestation_test

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/vectorial-dua/avlp/pkg/livestation"
	"github.com/vectorial-dua/avlp/pkg/rag"
	"github.com/vectorial-dua/avlp/pkg/vector"
)

func TestGenerateLiveStationWithSources(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	kb := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "data", "knowledge_base"))
	store := rag.NewStore()
	emb := rag.DefaultEmbedder()
	if _, err := rag.IngestWalk(context.Background(), store, rag.IngestOptions{Root: kb, Embedder: emb}); err != nil {
		t.Fatal(err)
	}
	idx := vector.NewIndex()
	gen := &livestation.Generator{Retriever: rag.NewRetriever(store, emb, 3), Nodes: idx}
	res, err := gen.Generate(context.Background(), livestation.Request{
		StudentID:      "s1",
		DoubtText:      "qué es un archivo .env y variables de entorno",
		QueryEmbedding: []float32{0.02, 0.03, 0.01, 0.04, 0.95},
		Frustration:    0.8,
		Dimension:      "Representacion",
		Format:         "conceptual",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !vector.ValidateNodeID(res.Node.ID) {
		t.Fatalf("bad node id %s", res.Node.ID)
	}
	if len(res.Sources) == 0 || res.Content == "" {
		t.Fatalf("expected sources and content, got %+v", res.Sources)
	}
	if idx.Len() != 1 {
		t.Fatalf("expected node registered, len=%d", idx.Len())
	}
}
