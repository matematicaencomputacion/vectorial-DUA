package rag_test

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/vectorial-dua/avlp/pkg/rag"
)

func kbRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	// pkg/rag -> repo root
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "data", "knowledge_base"))
}

func TestIngestAndRetrieveEnvDoc(t *testing.T) {
	store := rag.NewStore()
	emb := rag.NewHashEmbedder(64)
	n, err := rag.IngestWalk(context.Background(), store, rag.IngestOptions{Root: kbRoot(t), Embedder: emb})
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 || store.Len() == 0 {
		t.Fatalf("expected chunks, got n=%d len=%d", n, store.Len())
	}

	ret := rag.NewRetriever(store, emb, 3)
	hits, err := ret.RetrieveText(context.Background(), "variables de entorno archivo .env secretos dotenv Henry")
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("expected retrieval hits")
	}
	found := false
	for _, h := range hits {
		if filepath.ToSlash(h.Chunk.Source) == "henry/env-variables.md" ||
			filepath.Base(h.Chunk.Source) == "env-variables.md" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected env-variables in top hits, got %+v", rag.Sources(hits))
	}
}

func TestRetrieveMinSimilarityDropsOffTopic(t *testing.T) {
	store := rag.NewStore()
	emb := rag.NewHashEmbedder(64)
	if _, err := rag.IngestWalk(context.Background(), store, rag.IngestOptions{Root: kbRoot(t), Embedder: emb}); err != nil {
		t.Fatal(err)
	}

	ret := rag.NewRetriever(store, emb, 5)
	// The chunk simmatrix shows overlap between the weakest on-topic PostGIS
	// case and this off-topic control. This explicit stricter floor verifies
	// filtering without pretending one hash threshold separates every case.
	ret.MinSimilarity = 0.47

	off, err := ret.RetrieveText(context.Background(), "que es un bit")
	if err != nil {
		t.Fatal(err)
	}
	if len(off) != 0 {
		t.Fatalf("off-topic should yield zero hits above floor %.2f, got %d: %+v",
			ret.MinSimilarity, len(off), rag.Sources(off))
	}

	on, err := ret.RetrieveText(context.Background(), "variables de entorno archivo .env secretos dotenv Henry")
	if err != nil {
		t.Fatal(err)
	}
	if len(on) == 0 {
		t.Fatal("on-topic .env query must keep hits above the floor")
	}
	for _, h := range on {
		if h.Similarity < rag.DefaultMinSimilarity {
			t.Fatalf("hit below floor: sim=%.4f source=%s", h.Similarity, h.Chunk.Source)
		}
	}
}

func TestRetrieveVariablesYEscopesHitsEnvReference(t *testing.T) {
	store := rag.NewStore()
	emb := rag.NewHashEmbedder(64)
	if _, err := rag.IngestWalk(context.Background(), store, rag.IngestOptions{
		Root:     kbRoot(t),
		Embedder: emb,
	}); err != nil {
		t.Fatal(err)
	}
	hits, err := rag.NewRetriever(store, emb, 3).RetrieveText(context.Background(), "variables y escopes")
	if err != nil {
		t.Fatal(err)
	}
	for _, hit := range hits {
		if filepath.Base(hit.Chunk.Source) == "env-variables.md" {
			return
		}
	}
	t.Fatalf("expected env-variables reference, got %+v", rag.Sources(hits))
}

func TestHashEmbedderDeterministic(t *testing.T) {
	emb := rag.NewHashEmbedder(32)
	a, err := emb.Embed(context.Background(), "postgis ST_DWithin")
	if err != nil {
		t.Fatal(err)
	}
	b, err := emb.Embed(context.Background(), "postgis ST_DWithin")
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != 32 {
		t.Fatalf("dims=%d", len(a))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatal("expected deterministic embeddings")
		}
	}
}
