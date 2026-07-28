package evals_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/vectorial-dua/avlp/harness/evals"
	"github.com/vectorial-dua/avlp/harness/telemetry"
	"github.com/vectorial-dua/avlp/pkg/livestation"
	"github.com/vectorial-dua/avlp/pkg/rag"
	"github.com/vectorial-dua/avlp/pkg/vector"
)

func repoPath(t *testing.T, parts ...string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	base := filepath.Join(filepath.Dir(file), "..", "..")
	return filepath.Clean(filepath.Join(append([]string{base}, parts...)...))
}

type liveBridge struct{ g *livestation.Generator }

func (b liveBridge) GenerateLive(ctx context.Context, req vector.LiveRequest) (vector.LiveResult, error) {
	res, err := b.g.Generate(ctx, livestation.Request{
		StudentID: req.StudentID, DoubtText: req.DoubtText, QueryEmbedding: req.QueryEmbedding,
		Frustration: req.Frustration, Dimension: req.Dimension, Format: req.Format, TrackingULID: req.TrackingULID,
	})
	if err != nil {
		return vector.LiveResult{}, err
	}
	return vector.LiveResult{Node: res.Node, Content: res.Content, Sources: res.Sources, TrackingULID: res.TrackingULID}, nil
}

func TestGoldenRoutingEvalsPass(t *testing.T) {
	emb, err := evals.ResolveEmbedder("hash")
	if err != nil {
		t.Fatal(err)
	}
	idx := vector.NewIndexWithDims(emb.Dims())
	if err := vector.SeedDemoNodes(idx, emb); err != nil {
		t.Fatal(err)
	}
	store := rag.NewStore()
	if _, err := rag.IngestWalk(context.Background(), store, rag.IngestOptions{Root: repoPath(t, "data", "knowledge_base"), Embedder: emb}); err != nil {
		t.Fatal(err)
	}
	router := vector.NewRouter(idx, vector.NewEventBus())
	router.Live = liveBridge{g: &livestation.Generator{Retriever: rag.NewRetriever(store, emb, 3), Nodes: idx}}

	cases, err := evals.LoadCases(repoPath(t, "harness", "evals", "cases", "routing_golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	report := (&evals.Runner{Router: router, Tel: telemetry.NewCollector(), Embedder: emb}).Run(cases)
	if report.FailedCases != 0 {
		for _, r := range report.Results {
			if !r.Passed {
				t.Logf("FAIL %s: %s score=%.3f actual=%s sim=%.4f", r.CaseID, r.Message, r.AggregateScore, r.ActualOutcome, r.SimilarityScore)
			}
		}
		t.Fatalf("expected all golden cases to pass, failed=%d", report.FailedCases)
	}
}

func TestResolveEmbedderModes(t *testing.T) {
	h, err := evals.ResolveEmbedder("hash")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := h.(*rag.HashEmbedder); !ok {
		t.Fatalf("hash mode: got %T", h)
	}
	if _, err := evals.ResolveEmbedder("bogus"); err == nil {
		t.Fatal("expected error for unknown mode")
	}
	t.Setenv("AVLP_EMBEDDING_URL", "")
	if _, err := evals.ResolveEmbedder("env"); err == nil {
		t.Fatal("env mode without URL should error")
	}
}

func TestRAGFaithfulnessGolden(t *testing.T) {
	store := rag.NewStore()
	emb := rag.NewHashEmbedder(rag.DefaultEmbedDims)
	if _, err := rag.IngestWalk(context.Background(), store, rag.IngestOptions{Root: repoPath(t, "data", "knowledge_base"), Embedder: emb}); err != nil {
		t.Fatal(err)
	}
	idx := vector.NewIndexWithDims(emb.Dims())
	gen := &livestation.Generator{Retriever: rag.NewRetriever(store, emb, 3), Nodes: idx}

	raw, err := os.ReadFile(repoPath(t, "harness", "evals", "cases", "rag_golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []evals.RAGCase
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}

	failed := 0
	for _, c := range cases {
		q, err := emb.Embed(context.Background(), c.DoubtText)
		if err != nil {
			t.Fatalf("%s: embed: %v", c.CaseID, err)
		}
		res, err := gen.Generate(context.Background(), livestation.Request{
			StudentID: "rag-eval", DoubtText: c.DoubtText, Frustration: 0.7,
			Dimension: "Representacion", Format: "conceptual",
			QueryEmbedding: q,
		})
		if err != nil {
			t.Fatalf("%s: %v", c.CaseID, err)
		}
		sig := evals.ScoreRAGFaithfulness(res.Content, res.Retrieved, c.ExpectedSourceSubstr)
		if sig.Aggregate < 0.80 {
			failed++
			t.Logf("FAIL %s agg=%.3f faith=%.3f rel=%.3f sources=%v", c.CaseID, sig.Aggregate, sig.Faithfulness, sig.ContextRelevance, res.Sources)
		}
	}
	if failed > 0 {
		t.Fatalf("%d rag faithfulness cases failed", failed)
	}
}
