package vector_test

import (
	"testing"

	"github.com/vectorial-dua/avlp/pkg/vector"
)

func TestResolveThresholdUsesRequestWhenValid(t *testing.T) {
	if got := vector.ResolveThreshold(0.6); got != 0.6 {
		t.Fatalf("got %v", got)
	}
}

func TestResolveThresholdFallsBackToDefault(t *testing.T) {
	t.Setenv("AVLP_SIMILARITY_THRESHOLD", "")
	if got := vector.ResolveThreshold(0); got != vector.DefaultSimilarityThreshold {
		t.Fatalf("got %v want %v", got, vector.DefaultSimilarityThreshold)
	}
}

func TestEffectiveDefaultThresholdFromEnv(t *testing.T) {
	t.Setenv("AVLP_SIMILARITY_THRESHOLD", "0.6")
	if got := vector.EffectiveDefaultThreshold(); got != 0.6 {
		t.Fatalf("got %v", got)
	}
	if got := vector.ResolveThreshold(0); got != 0.6 {
		t.Fatalf("ResolveThreshold(0)=%v want 0.6", got)
	}
}

func TestEffectiveDefaultThresholdRejectsInvalid(t *testing.T) {
	t.Setenv("AVLP_SIMILARITY_THRESHOLD", "1.5")
	if got := vector.EffectiveDefaultThreshold(); got != vector.DefaultSimilarityThreshold {
		t.Fatalf("got %v", got)
	}
}
