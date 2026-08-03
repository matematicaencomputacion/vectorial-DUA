package bench

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSynthEmbeddingDeterministic(t *testing.T) {
	a := synthEmbedding(DefaultSeed, 64, 42)
	b := synthEmbedding(DefaultSeed, 64, 42)
	if len(a) != 64 {
		t.Fatalf("dims=%d", len(a))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("embedding not deterministic at %d", i)
		}
	}
	c := synthEmbedding(DefaultSeed, 64, 43)
	same := true
	for i := range a {
		if a[i] != c[i] {
			same = false
			break
		}
	}
	if same {
		t.Fatal("distinct indices must differ")
	}
}

func TestSynthNodeIDValid(t *testing.T) {
	for _, i := range []int{0, 1, 999, 100_000} {
		id := synthNodeID(i)
		if !strings.HasPrefix(id, "dua::Bench::basico::practica::") {
			t.Fatalf("id=%s", id)
		}
		ulid := strings.TrimPrefix(id, "dua::Bench::basico::practica::")
		if len(ulid) != 26 {
			t.Fatalf("ulid len=%d for i=%d", len(ulid), i)
		}
	}
}

func TestRunCISizesUnderThreshold(t *testing.T) {
	rep, err := Run(Config{Sizes: CISizes, Dims: DefaultDims, Seed: DefaultSeed})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Scenarios) != len(CISizes)*len(DefaultDims)*2 {
		t.Fatalf("scenarios=%d", len(rep.Scenarios))
	}
	for _, s := range rep.Scenarios {
		if !s.CIGuard {
			t.Fatalf("expected CIGuard for N=%d", s.N)
		}
		if s.HumanLine == "" || !strings.Contains(s.HumanLine, "umbral ADR-001") {
			t.Fatalf("human line: %q", s.HumanLine)
		}
	}
	// testing.Benchmark under -race inflates Nearest/Retrieve ~10×; the real
	// algorithmic guard is the CI step `go run … -bench-sizes 100,1000` (no race).
	if underRace {
		t.Log("skip CIGuard assert under -race (timing not comparable to ADR-001)")
		return
	}
	if rep.CIGuardFailed {
		t.Fatalf("CI guard failed on fresh code:\n%s", dumpLines(rep))
	}
}

func TestWriteJSON(t *testing.T) {
	rep, err := Run(Config{Sizes: []int{100}, Dims: []int{64}, Seed: DefaultSeed})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path, err := WriteJSON(dir, rep)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "bench.json" {
		t.Fatalf("path=%s", path)
	}
}

func dumpLines(rep Report) string {
	var b strings.Builder
	FormatTable(&b, rep)
	return b.String()
}
