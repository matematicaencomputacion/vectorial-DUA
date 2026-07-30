package livestation_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/vectorial-dua/avlp/harness/telemetry"
	"github.com/vectorial-dua/avlp/pkg/livestation"
	"github.com/vectorial-dua/avlp/pkg/rag"
	"github.com/vectorial-dua/avlp/pkg/rogerian"
	"github.com/vectorial-dua/avlp/pkg/vector"
)

type stubSynthesizer struct {
	content string
	err     error
	bundle  rogerian.PromptBundle
}

func (s *stubSynthesizer) Synthesize(_ context.Context, bundle rogerian.PromptBundle) (string, error) {
	s.bundle = bundle
	return s.content, s.err
}

func (*stubSynthesizer) ModelName() string { return "stub-generative" }

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

func TestGenerateUsesSynthesizerAndAppendsCanonicalSources(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	kb := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "data", "knowledge_base"))
	store := rag.NewStore()
	emb := rag.NewHashEmbedder(64)
	if _, err := rag.IngestWalk(context.Background(), store, rag.IngestOptions{Root: kb, Embedder: emb}); err != nil {
		t.Fatal(err)
	}
	synth := &stubSynthesizer{
		content: "# Explicación adaptada\n\nUn archivo .env separa configuración [1].\n\n## Fuentes\n- inventada.md",
	}
	tel := telemetry.NewCollector()
	gen := &livestation.Generator{
		Retriever:   rag.NewRetriever(store, emb, 3),
		Nodes:       vector.NewIndex(),
		Synthesizer: synth,
		Tel:         tel,
	}
	res, err := gen.Generate(context.Background(), livestation.Request{
		StudentID: "s-generative",
		DoubtText: "qué es un archivo .env y variables de entorno",
		Dimension: "Representacion",
		Format:    "conceptual",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Content, "Explicación adaptada") {
		t.Fatalf("content=%q", res.Content)
	}
	if strings.Contains(res.Content, "inventada.md") || strings.Count(res.Content, "## Fuentes") != 1 {
		t.Fatalf("sources were not canonicalized:\n%s", res.Content)
	}
	if !strings.Contains(res.Content, "env-variables.md") {
		t.Fatalf("missing retrieved source:\n%s", res.Content)
	}
	if synth.bundle.FullPrompt == "" || !strings.Contains(synth.bundle.FullPrompt, "Contexto verificado") {
		t.Fatalf("bundle=%+v", synth.bundle)
	}
	snapshot := tel.Snapshot()
	if len(snapshot.LLMSpans) != 1 || snapshot.LLMSpans[0].Model != "stub-generative" ||
		!snapshot.LLMSpans[0].Success {
		t.Fatalf("spans=%+v", snapshot.LLMSpans)
	}
}

func TestGenerateLogsAndFallsBackWhenSynthesizerFails(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	kb := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "data", "knowledge_base"))
	store := rag.NewStore()
	emb := rag.NewHashEmbedder(64)
	if _, err := rag.IngestWalk(context.Background(), store, rag.IngestOptions{Root: kb, Embedder: emb}); err != nil {
		t.Fatal(err)
	}
	var logs string
	tel := telemetry.NewCollector()
	gen := &livestation.Generator{
		Retriever:   rag.NewRetriever(store, emb, 3),
		Nodes:       vector.NewIndex(),
		Synthesizer: &stubSynthesizer{err: errors.New("backend unavailable")},
		Tel:         tel,
		Logf: func(format string, args ...any) {
			logs += fmt.Sprintf(format, args...)
		},
	}
	res, err := gen.Generate(context.Background(), livestation.Request{
		StudentID: "s-fallback",
		DoubtText: "qué es un archivo .env y variables de entorno",
		Dimension: "Representacion",
		Format:    "conceptual",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Content, "Explicación anclada al contexto") ||
		!strings.Contains(res.Content, "## Fuentes") {
		t.Fatalf("expected extractive fallback:\n%s", res.Content)
	}
	if !strings.Contains(logs, "extractive fallback") || !strings.Contains(logs, "backend unavailable") {
		t.Fatalf("logs=%q", logs)
	}
	snapshot := tel.Snapshot()
	if snapshot.Counters["livestation_synthesis_fallback_total"] != 1 {
		t.Fatalf("counters=%v", snapshot.Counters)
	}
	if len(snapshot.LLMSpans) != 1 || snapshot.LLMSpans[0].Success ||
		!strings.Contains(snapshot.LLMSpans[0].ErrorMessage, "backend unavailable") {
		t.Fatalf("spans=%+v", snapshot.LLMSpans)
	}
}

func TestGenerateWithConfiguredLLMIntegration(t *testing.T) {
	// TestMain clears AVLP_* for hermetic CI; opt in with a non-AVLP flag.
	if os.Getenv("RUN_LLM_INTEGRATION") == "" {
		t.Skip("set RUN_LLM_INTEGRATION=1 to exercise a local Chat Completions backend")
	}
	if os.Getenv("AVLP_LLM_URL") == "" {
		t.Setenv("AVLP_LLM_URL", "http://localhost:11434/v1")
	}
	if os.Getenv("AVLP_LLM_MODEL") == "" {
		t.Setenv("AVLP_LLM_MODEL", "qwen3:4b-instruct")
	}
	synthesizer, err := rogerian.NewHTTPSynthesizerFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if synthesizer == nil {
		t.Fatal("expected configured synthesizer")
	}
	_, file, _, _ := runtime.Caller(0)
	kb := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "data", "knowledge_base"))
	store := rag.NewStore()
	emb := rag.NewHashEmbedder(64)
	if _, err := rag.IngestWalk(context.Background(), store, rag.IngestOptions{Root: kb, Embedder: emb}); err != nil {
		t.Fatal(err)
	}
	gen := &livestation.Generator{
		Retriever:   rag.NewRetriever(store, emb, 3),
		Nodes:       vector.NewIndex(),
		Synthesizer: synthesizer,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	res, err := gen.Generate(ctx, livestation.Request{
		StudentID:   "ollama-integration",
		DoubtText:   "No entiendo qué es un archivo .env ni para qué sirven las variables de entorno",
		Frustration: 0.7,
		Dimension:   "Representacion",
		Format:      "conceptual",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Sources) == 0 || !strings.Contains(res.Content, "## Fuentes") ||
		!strings.Contains(res.Content, "env-variables.md") {
		t.Fatalf("generated station is not grounded:\n%s", res.Content)
	}
	t.Logf("generated station:\n%s", res.Content)
}
