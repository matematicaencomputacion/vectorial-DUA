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

// TestVariablesYEscopesTypoDoesNotFalseMatchHash documents the honest hash
// behavior of the typo query "variables y escopes".
//
// HashEmbedder is lexical: after doc-expansion Ola 2.b the nearest interactive
// seed is sometimes debug-emergency (~0.21), not variables-scope — both under
// the hash-calibrated floor (0.482, calibrate 2026-08-03). Asserting a confident
// static match to scope here would be a capacity hash no longer has (it used to
// pass by accidental token overlap on "variables"). The correct hash outcome is
// the live/miss path: similarity stays below the calibrated threshold.
//
// The semantic claim ("variables y escopes" → nodo de scope) lives in
// harness/evals/cases/routing_golden.json as variables-escopes-typo-static
// (expected_outcome=static, expected_outcome_hash=live). Reference scores for
// future readers: hash nearest ~0.21; bge-m3 ~0.53 (run -embedder env locally).
func TestVariablesYEscopesTypoDoesNotFalseMatchHash(t *testing.T) {
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
	// Umbral hash calibrado 2026-08-03: go run ./cmd/harness -suite calibrate -embedder hash
	const hashCalibratedThreshold float32 = 0.482
	router := vector.NewRouter(idx, vector.NewEventBus())
	outcome, err := router.QueryNearest("stu-typo", query, hashCalibratedThreshold)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Matched && !outcome.IsLiveGenerated {
		t.Fatalf("hash debe rechazar un match estático confiado para el typo (sim=%.4f thr=%.3f nodo=%s); el camino correcto es miss/live",
			outcome.Similarity, hashCalibratedThreshold, outcome.Node.ID)
	}
	if outcome.Similarity >= hashCalibratedThreshold {
		t.Fatalf("sim=%.4f no debe alcanzar el umbral calibrado %.3f (hash ~0.21 tipográfico)",
			outcome.Similarity, hashCalibratedThreshold)
	}
}
