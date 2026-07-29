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
