package evals_test

import (
	"context"
	"strings"
	"testing"

	"github.com/vectorial-dua/avlp/harness/evals"
	"github.com/vectorial-dua/avlp/internal/testenv"
	"github.com/vectorial-dua/avlp/pkg/rag"
	"github.com/vectorial-dua/avlp/pkg/vector"
)

func TestBuildSimMatrixHash(t *testing.T) {
	testenv.Isolate(t)

	emb := rag.NewHashEmbedder(rag.DefaultEmbedDims)
	idx := vector.NewIndexWithDims(emb.Dims())
	if err := vector.SeedDemoNodes(idx, emb); err != nil {
		t.Fatal(err)
	}
	cases := []evals.Case{{
		CaseID:          "probe",
		QueryText:       "¿por qué debería importarme configurar el .env?",
		ExpectedOutcome: "static",
	}}
	report, err := evals.BuildSimMatrix(context.Background(), cases, idx, emb, "hash")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Rows) != 1 {
		t.Fatalf("rows=%d", len(report.Rows))
	}
	if len(report.Rows[0].Cells) != idx.Len() {
		t.Fatalf("cells=%d want %d", len(report.Rows[0].Cells), idx.Len())
	}
	nearest := 0
	for _, c := range report.Rows[0].Cells {
		if c.IsNearest {
			nearest++
		}
	}
	if nearest != 1 {
		t.Fatalf("nearest marks=%d", nearest)
	}
	table := evals.FormatSimMatrixTable(report)
	if !strings.Contains(table, "probe") || !strings.Contains(table, "*") {
		t.Fatalf("unexpected table:\n%s", table)
	}
}

func TestBuildChunkSimMatrixVariablesEscopesReference(t *testing.T) {
	testenv.Isolate(t)
	emb := rag.NewHashEmbedder(rag.DefaultEmbedDims)
	store := rag.NewStore()
	if _, err := rag.IngestWalk(context.Background(), store, rag.IngestOptions{
		Root:     repoPath(t, "data", "knowledge_base"),
		Embedder: emb,
	}); err != nil {
		t.Fatal(err)
	}
	cases, err := evals.LoadChunkSimCases(
		repoPath(t, "harness", "evals", "cases", "rag_simmatrix.json"),
	)
	if err != nil {
		t.Fatal(err)
	}
	report, err := evals.BuildChunkSimMatrix(context.Background(), cases, store, emb, "hash")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Rows) != len(cases) || len(report.Rows[0].Cells) != store.Len() {
		t.Fatalf("report rows=%d cells=%d chunks=%d", len(report.Rows), len(report.Rows[0].Cells), store.Len())
	}
	var typoRow *evals.ChunkSimRow
	for i := range report.Rows {
		if report.Rows[i].CaseID == "rag-typo-variables-escopes" {
			typoRow = &report.Rows[i]
			break
		}
	}
	if typoRow == nil {
		t.Fatal("missing variables y escopes calibration row")
	}
	foundEnv := false
	for _, cell := range typoRow.Cells {
		if cell.IsExpectedSource {
			foundEnv = true
			break
		}
	}
	if !foundEnv {
		t.Fatal("variables y escopes row has no env-variables reference chunk")
	}
	if report.SuggestedMinSimilarity <= 0 || report.SuggestedMinSimilarity > 1 {
		t.Fatalf("suggested RAG floor=%v", report.SuggestedMinSimilarity)
	}
	table := evals.FormatChunkSimMatrixTable(report)
	if !strings.Contains(table, "rag-typo-variables-escopes") {
		t.Fatalf("unexpected table:\n%s", table)
	}
}
