package testenv_test

import (
	"os"
	"testing"

	"github.com/vectorial-dua/avlp/internal/testenv"
	"github.com/vectorial-dua/avlp/pkg/vector"
)

func TestIsolateHidesExportedThresholdAndRestoresIt(t *testing.T) {
	t.Setenv("AVLP_SIMILARITY_THRESHOLD", "0.55")

	t.Run("isolated", func(t *testing.T) {
		testenv.Isolate(t)
		if _, ok := os.LookupEnv("AVLP_SIMILARITY_THRESHOLD"); ok {
			t.Fatal("AVLP_SIMILARITY_THRESHOLD should be unset, not empty")
		}
		if got := vector.EffectiveDefaultThreshold(); got != vector.DefaultSimilarityThreshold {
			t.Fatalf("threshold under isolation: got %v, want %v", got, vector.DefaultSimilarityThreshold)
		}
	})

	if got := os.Getenv("AVLP_SIMILARITY_THRESHOLD"); got != "0.55" {
		t.Fatalf("value not restored after cleanup: got %q", got)
	}
}

func TestIsolateCoversAnyPrefixedVariable(t *testing.T) {
	t.Setenv("AVLP_FUTURE_KNOB", "surprise")
	testenv.Isolate(t)
	if _, ok := os.LookupEnv("AVLP_FUTURE_KNOB"); ok {
		t.Fatal("Isolate must clear variables it does not know about")
	}
}
