package vector_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vectorial-dua/avlp/internal/testenv"
	"github.com/vectorial-dua/avlp/pkg/vector"
)

func TestResolveEffectiveThresholdPrecedence(t *testing.T) {
	testenv.Isolate(t)
	path := filepath.Join(t.TempDir(), "avlp.json")
	if err := vector.WriteThresholdConfig(path, 0.55); err != nil {
		t.Fatal(err)
	}

	fromFile := vector.ResolveEffectiveThreshold(path)
	if fromFile.Value != 0.55 || fromFile.Source != vector.ThresholdSourceFile {
		t.Fatalf("from file=%+v", fromFile)
	}

	t.Setenv("AVLP_SIMILARITY_THRESHOLD", "0.62")
	fromEnv := vector.ResolveEffectiveThreshold(path)
	if fromEnv.Value != 0.62 || fromEnv.Source != vector.ThresholdSourceEnv {
		t.Fatalf("from env=%+v", fromEnv)
	}
}

func TestResolveEffectiveThresholdFallsBackForInvalidSources(t *testing.T) {
	testenv.Isolate(t)
	path := filepath.Join(t.TempDir(), "avlp.json")
	if err := os.WriteFile(path, []byte(`{"version":2,"similarity_threshold":0.5}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AVLP_SIMILARITY_THRESHOLD", "not-a-number")

	got := vector.ResolveEffectiveThreshold(path)
	if got.Value != vector.DefaultSimilarityThreshold || got.Source != vector.ThresholdSourceDefault {
		t.Fatalf("resolution=%+v", got)
	}
}

func TestWriteThresholdConfigRoundTripAndValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "avlp.json")
	if err := vector.WriteThresholdConfig(path, 0.58); err != nil {
		t.Fatal(err)
	}
	got, err := vector.ReadThresholdConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != 0.58 {
		t.Fatalf("threshold=%v want=0.58", got)
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".avlp-config-*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files remain: %v", matches)
	}
	for _, invalid := range []float32{0, -0.1, 1.1} {
		if err := vector.WriteThresholdConfig(path, invalid); err == nil {
			t.Fatalf("threshold %v should fail", invalid)
		}
	}
}
