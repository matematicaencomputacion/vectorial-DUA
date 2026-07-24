package rogerian_test

import (
	"testing"

	"github.com/vectorial-dua/avlp/pkg/rag"
	"github.com/vectorial-dua/avlp/pkg/rogerian"
)

func TestPromptBuilderIncludesContextAndTone(t *testing.T) {
	chunks := []rag.ScoredChunk{{
		Chunk: rag.Chunk{
			Source: "henry/env-variables.md",
			Title:  "Variables de entorno",
			Text:   "Un archivo .env guarda secretos fuera del código.",
		},
		Similarity: 0.9,
	}}
	p := (rogerian.PromptBuilder{}).Build(rogerian.BuildInput{
		DoubtText:   "No entiendo qué es un .env",
		Frustration: 0.8,
		Dimension:   "Representacion",
		Format:      "conceptual",
		Chunks:      chunks,
	})
	if p.Tone != rogerian.ToneValidate {
		t.Fatalf("tone=%s", p.Tone)
	}
	if len(p.Sources) != 1 || p.Sources[0] != "henry/env-variables.md" {
		t.Fatalf("sources=%v", p.Sources)
	}
	if p.Context == "" || p.FullPrompt == "" {
		t.Fatal("expected context and full prompt")
	}
}
