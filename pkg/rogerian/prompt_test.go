package rogerian_test

import (
	"strings"
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
	if p.EmptyContext {
		t.Fatal("expected non-empty context branch")
	}
}

func TestPromptBuilderEmptyContextAvoidsInternalScaffold(t *testing.T) {
	p := (rogerian.PromptBuilder{}).Build(rogerian.BuildInput{
		DoubtText:       "qué es un quark charm en física de partículas",
		Frustration:     0.2,
		Dimension:       "Representacion",
		Format:          "conceptual",
		Chunks:          nil,
		AvailableTopics: []string{"Variables y Scope", "async/await", "PostGIS"},
	})
	if !p.EmptyContext {
		t.Fatal("expected empty-context branch")
	}
	if len(p.Sources) != 0 {
		t.Fatalf("sources=%v", p.Sources)
	}
	if !strings.Contains(strings.ToLower(p.SystemStyle), "prohibido") {
		t.Fatal("expected explicit prohibitions in system style")
	}
	instr := strings.ToLower(p.FullPrompt)
	if i := strings.Index(instr, "## instrucciones"); i >= 0 {
		instr = instr[i:]
	}
	for _, bad := range []string{"micro-desafío", "contexto verificado", "cita las fuentes", "formato dua"} {
		if strings.Contains(instr, bad) {
			t.Fatalf("empty prompt instructions leaked %q:\n%s", bad, p.FullPrompt)
		}
	}
	if !strings.Contains(p.FullPrompt, "Variables y Scope") {
		t.Fatalf("expected concrete topic hints:\n%s", p.FullPrompt)
	}
	if !strings.Contains(p.FullPrompt, "2 a 4 frases") {
		t.Fatalf("expected length instruction:\n%s", p.FullPrompt)
	}
}
