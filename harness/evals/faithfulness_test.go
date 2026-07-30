package evals_test

import (
	"testing"

	"github.com/vectorial-dua/avlp/harness/evals"
	"github.com/vectorial-dua/avlp/pkg/rag"
)

func generativeFixture() []rag.ScoredChunk {
	return []rag.ScoredChunk{{
		Chunk: rag.Chunk{
			Source: "henry/env-variables.md",
			Title:  "Variables de entorno",
			Text:   "Un archivo env separa configuración y secretos del código. Las variables de entorno evitan publicar credenciales.",
		},
		Similarity: 0.9,
	}}
}

func TestGenerativeFaithfulnessRewardsGroundedTermsAndSource(t *testing.T) {
	answer := `El archivo env separa configuración, secretos y credenciales del código.
Las variables de entorno ayudan a evitar publicar secretos [1].

## Fuentes
- [1] henry/env-variables.md`
	signals := evals.ScoreRAGFaithfulnessMode(
		answer,
		generativeFixture(),
		"env-variables",
		evals.FaithfulnessGenerative,
	)
	if signals.GroundedPrecision < 0.8 || signals.ContextCoverage < 0.8 ||
		signals.SourceAttribution != 1 || signals.Aggregate < 0.85 {
		t.Fatalf("signals=%+v", signals)
	}
}

func TestGenerativeFaithfulnessPenalizesUnsupportedClaimsDespiteCitation(t *testing.T) {
	answer := `La computación cuántica garantiza velocidad infinita y elimina cualquier error.

## Fuentes
- [1] henry/env-variables.md`
	signals := evals.ScoreRAGFaithfulnessMode(
		answer,
		generativeFixture(),
		"env-variables",
		evals.FaithfulnessGenerative,
	)
	if signals.SourceAttribution != 1 {
		t.Fatalf("source=%v", signals.SourceAttribution)
	}
	if signals.GroundedPrecision > 0.25 || signals.ContextCoverage > 0.25 || signals.Aggregate >= 0.5 {
		t.Fatalf("unsupported answer scored too high: %+v", signals)
	}
}

func TestExtractiveFaithfulnessModeRemainsCompatible(t *testing.T) {
	chunks := generativeFixture()
	answer := chunks[0].Chunk.Text + "\n\n## Fuentes\n- [1] henry/env-variables.md"
	legacy := evals.ScoreRAGFaithfulness(answer, chunks, "env-variables")
	explicit := evals.ScoreRAGFaithfulnessMode(
		answer,
		chunks,
		"env-variables",
		evals.FaithfulnessExtractive,
	)
	if legacy != explicit || legacy.Aggregate < 0.8 {
		t.Fatalf("legacy=%+v explicit=%+v", legacy, explicit)
	}
}
