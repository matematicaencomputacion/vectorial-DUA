package vector_test

import (
	"context"
	"strings"
	"testing"

	"github.com/vectorial-dua/avlp/pkg/rag"
	"github.com/vectorial-dua/avlp/pkg/vector"
)

func TestContentEmbedDimsAlignedWithRAG(t *testing.T) {
	if vector.ContentEmbedDims != rag.DefaultEmbedDims {
		t.Fatalf("ContentEmbedDims=%d rag.DefaultEmbedDims=%d; content spaces must match",
			vector.ContentEmbedDims, rag.DefaultEmbedDims)
	}
	emb := rag.DefaultEmbedder()
	if emb.Dims() != vector.ContentEmbedDims {
		t.Fatalf("DefaultEmbedder dims=%d want %d", emb.Dims(), vector.ContentEmbedDims)
	}
}

func TestFitContentEmbeddingPadsLegacyShort(t *testing.T) {
	short := []float32{0.92, 0.10, 0.05, 0.20, 0.15}
	got, err := vector.FitContentEmbedding(short)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != vector.ContentEmbedDims {
		t.Fatalf("len=%d want %d", len(got), vector.ContentEmbedDims)
	}
	for i, v := range short {
		if got[i] != v {
			t.Fatalf("pad altered head[%d]: got %v want %v", i, got[i], v)
		}
	}
	for i := len(short); i < len(got); i++ {
		if got[i] != 0 {
			t.Fatalf("expected zero pad at %d", i)
		}
	}
}

func TestFitContentEmbeddingRejectsOversized(t *testing.T) {
	tooLong := make([]float32, vector.ContentEmbedDims+1)
	_, err := vector.FitContentEmbedding(tooLong)
	if err == nil {
		t.Fatal("expected error for oversized embedding")
	}
	if !strings.Contains(err.Error(), "refusing silent truncate") {
		t.Fatalf("expected truncate refusal message, got %v", err)
	}
}

func TestIndexUpsertRejectsDimMismatch(t *testing.T) {
	idx := vector.NewIndex()
	id, err := vector.NewNodeID("Representacion", "basico", "visual")
	if err != nil {
		t.Fatal(err)
	}
	err = idx.Upsert(vector.Node{
		ID:           id,
		DimensionDUA: "Representacion",
		Difficulty:   "basico",
		Format:       "visual",
		ResourceURL:  "master://x",
		Embedding:    []float32{1, 0, 0, 0, 0}, // 5 dims; index wants ContentEmbedDims
	})
	if err == nil {
		t.Fatal("expected dims mismatch error")
	}
	if !strings.Contains(err.Error(), "embedding dims mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "not V_e") {
		t.Fatalf("error should clarify content vs V_e: %v", err)
	}
}

func TestIndexUpsertAcceptsFittedContentDims(t *testing.T) {
	idx := vector.NewIndex()
	id, err := vector.NewNodeID("Accion", "basico", "practica")
	if err != nil {
		t.Fatal(err)
	}
	emb := vector.MustFitContentEmbedding([]float32{1, 0, 0, 0, 0})
	if err := idx.Upsert(vector.Node{
		ID:           id,
		DimensionDUA: "Accion",
		Difficulty:   "basico",
		Format:       "practica",
		ResourceURL:  "ide://x",
		Embedding:    emb,
	}); err != nil {
		t.Fatal(err)
	}
	if idx.Len() != 1 {
		t.Fatalf("len=%d", idx.Len())
	}
	if idx.Dims() != vector.ContentEmbedDims {
		t.Fatalf("index dims=%d", idx.Dims())
	}
}

func TestSeedDemoNodesShareContentDimsWithEmbedder(t *testing.T) {
	idx := vector.NewIndex()
	if err := vector.SeedDemoNodes(idx, rag.DefaultEmbedder()); err != nil {
		t.Fatal(err)
	}
	emb := rag.DefaultEmbedder()
	q, err := emb.Embed(context.Background(), "variables de entorno archivo .env")
	if err != nil {
		t.Fatal(err)
	}
	if len(q) != idx.Dims() {
		t.Fatalf("query dims=%d index dims=%d — query_text path would never match", len(q), idx.Dims())
	}
	match := idx.Nearest(q)
	if !match.Found {
		t.Fatal("expected nearest over non-empty index")
	}
	_ = match.Similarity
}
