package livestation_test

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
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
	if len(res.Node.Embedding) != vector.ContentEmbedDims {
		t.Fatalf("live node embedding dims=%d want %d", len(res.Node.Embedding), vector.ContentEmbedDims)
	}
}

func TestGenerateOffTopicYieldsHonestEmptySources(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	kb := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "data", "knowledge_base"))
	store := rag.NewStore()
	emb := rag.NewHashEmbedder(64)
	if _, err := rag.IngestWalk(context.Background(), store, rag.IngestOptions{Root: kb, Embedder: emb}); err != nil {
		t.Fatal(err)
	}
	idx := vector.NewIndex()
	ret := rag.NewRetriever(store, emb, 5)
	ret.MinSimilarity = rag.DefaultMinSimilarity
	gen := &livestation.Generator{Retriever: ret, Nodes: idx}
	res, err := gen.Generate(context.Background(), livestation.Request{
		StudentID:   "s-bit",
		DoubtText:   "que es un bit",
		Frustration: 0.4,
		Dimension:   "Representacion",
		Format:      "conceptual",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Sources) != 0 {
		t.Fatalf("expected no spurious sources, got %v", res.Sources)
	}
	if len(res.Retrieved) != 0 {
		t.Fatalf("expected no retrieved chunks, got %d", len(res.Retrieved))
	}
	if !strings.Contains(res.Content, "No encontré material verificado") {
		t.Fatalf("expected honest empty-KB copy, got %q", res.Content)
	}
}
