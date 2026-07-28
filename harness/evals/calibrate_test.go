package evals_test

import (
	"context"
	"testing"

	"github.com/vectorial-dua/avlp/harness/evals"
	"github.com/vectorial-dua/avlp/pkg/rag"
	"github.com/vectorial-dua/avlp/pkg/vector"
)

func TestBuildCalibrationHashSuggestsThreshold(t *testing.T) {
	emb := rag.NewHashEmbedder(rag.DefaultEmbedDims)
	idx := vector.NewIndexWithDims(emb.Dims())
	if err := vector.SeedDemoNodes(idx, emb); err != nil {
		t.Fatal(err)
	}
	cases, err := evals.LoadCases(repoPath(t, "harness", "evals", "cases", "routing_golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	matrix, err := evals.BuildSimMatrix(context.Background(), cases, idx, emb, "hash")
	if err != nil {
		t.Fatal(err)
	}
	rep := evals.BuildCalibration(matrix, cases)
	if rep.SuggestedThreshold <= 0 || rep.SuggestedThreshold > 1 {
		t.Fatalf("suggested=%v", rep.SuggestedThreshold)
	}
	if len(rep.Cases) == 0 {
		t.Fatal("expected cases")
	}
	out := evals.FormatCalibrationReport(rep)
	if out == "" {
		t.Fatal("empty format")
	}
	t.Log("\n" + out)
}
