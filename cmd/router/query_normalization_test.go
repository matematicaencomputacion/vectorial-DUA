package main

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/vectorial-dua/avlp/pkg/dua"
	"github.com/vectorial-dua/avlp/pkg/rag"
	"github.com/vectorial-dua/avlp/pkg/vector"
)

func TestVariablesYEscopesRoutesToVariablesScope(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	seeds := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "data", "nodes", "interactive"))
	reg := dua.NewRegistry()
	if _, err := reg.LoadDir(seeds); err != nil {
		t.Fatal(err)
	}
	emb := rag.NewHashEmbedder(rag.DefaultEmbedDims)
	idx := vector.NewIndexWithDims(emb.Dims())
	reg.ForEach(func(node *dua.InteractiveVideoNode) {
		if err := idx.Upsert(vector.Node{
			ID:           node.NodeID,
			DimensionDUA: node.DimensionDUA,
			Difficulty:   "basico",
			Format:       formatFromNodeID(node.NodeID),
			ResourceURL:  "interactive://" + node.NodeID,
			Embedding:    node.Embedding,
		}); err != nil {
			t.Errorf("upsert %s: %v", node.NodeID, err)
		}
	})
	query, err := emb.Embed(context.Background(), "variables y escopes")
	if err != nil {
		t.Fatal(err)
	}
	router := vector.NewRouter(idx, vector.NewEventBus())
	outcome, err := router.QueryNearest("stu-typo", query, 0.01)
	if err != nil {
		t.Fatal(err)
	}
	const variablesScopeID = "dua::Representacion::basico::visual::01ARZ3NDEKTSV4RRFFQ69G5FAV"
	if !outcome.Matched || outcome.Node.ID != variablesScopeID {
		t.Fatalf("outcome=%+v want node=%s", outcome, variablesScopeID)
	}
}
