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
