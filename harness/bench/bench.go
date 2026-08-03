// Package bench instruments Index.Nearest and rag.Store.Retrieve against
// synthetic corpora so ADR-001 §4 latency triggers become measured signals.
package bench

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/vectorial-dua/avlp/pkg/rag"
	"github.com/vectorial-dua/avlp/pkg/vector"
)

// ADR001LatencyThresholdMS is the §4 kernel-latency trigger (Nearest/Retrieve).
const ADR001LatencyThresholdMS = 5.0

// DefaultSeed makes embeddings and ULIDs reproducible across runs.
const DefaultSeed int64 = 20260803

// DefaultSizes is the full ladder from ADR-001 §4 / ola7-backlog.
var DefaultSizes = []int{100, 1_000, 10_000, 100_000}

// CISizes are the fast scenarios suitable for the GitHub Actions step.
var CISizes = []int{100, 1_000}

// DefaultDims covers offline hash (64) and a dense-embedder stand-in (1024).
var DefaultDims = []int{vector.ContentEmbedDims, 1024}

// Config selects which synthetic scales to run.
type Config struct {
	Sizes []int
	Dims  []int
	Seed  int64
}

// Scenario is one (op × N × dims) measurement.
type Scenario struct {
	Op             string  `json:"op"` // Nearest | Retrieve
	N              int     `json:"n"`
	Dims           int     `json:"dims"`
	NsPerOp        int64   `json:"ns_per_op"`
	AllocsPerOp    int64   `json:"allocs_per_op"`
	MsPerOp        float64 `json:"ms_per_op"`
	ThresholdMs    float64 `json:"threshold_ms"`
	PctOfThreshold float64 `json:"pct_of_threshold"`
	Crossed        bool    `json:"crossed"`
	CIGuard        bool    `json:"ci_guard"` // true when N is in the CI ladder
	HumanLine      string  `json:"human_line"`
	BenchmarkN     int     `json:"benchmark_n"`
}

// Report is written to harness/out/bench.json.
type Report struct {
	GeneratedAt   time.Time  `json:"generated_at"`
	Seed          int64      `json:"seed"`
	ThresholdMs   float64    `json:"adr001_threshold_ms"`
	Scenarios     []Scenario `json:"scenarios"`
	AnyCrossed    bool       `json:"any_crossed"`
	CIGuardFailed bool       `json:"ci_guard_failed"`
}

// Run executes programmatic testing.Benchmark scenarios.
func Run(cfg Config) (Report, error) {
	if cfg.Seed == 0 {
		cfg.Seed = DefaultSeed
	}
	if len(cfg.Sizes) == 0 {
		cfg.Sizes = append([]int(nil), DefaultSizes...)
	}
	if len(cfg.Dims) == 0 {
		cfg.Dims = append([]int(nil), DefaultDims...)
	}
	sizes := uniqueSorted(cfg.Sizes)
	dims := uniqueSorted(cfg.Dims)

	rep := Report{
		GeneratedAt: time.Now().UTC(),
		Seed:        cfg.Seed,
		ThresholdMs: ADR001LatencyThresholdMS,
	}

	for _, dimsN := range dims {
		for _, n := range sizes {
			nearest, err := benchNearest(cfg.Seed, n, dimsN)
			if err != nil {
				return Report{}, err
			}
			retrieve, err := benchRetrieve(cfg.Seed, n, dimsN)
			if err != nil {
				return Report{}, err
			}
			rep.Scenarios = append(rep.Scenarios, nearest, retrieve)
		}
	}

	for _, s := range rep.Scenarios {
		if s.Crossed {
			rep.AnyCrossed = true
		}
		if s.CIGuard && s.Crossed {
			rep.CIGuardFailed = true
		}
	}
	return rep, nil
}

func benchNearest(seed int64, n, dims int) (Scenario, error) {
	idx, query, err := buildIndex(seed, n, dims)
	if err != nil {
		return Scenario{}, err
	}
	res := testing.Benchmark(func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = idx.Nearest(query)
		}
	})
	return scenarioFrom("Nearest", n, dims, res), nil
}

func benchRetrieve(seed int64, n, dims int) (Scenario, error) {
	store, query := buildStore(seed, n, dims)
	res := testing.Benchmark(func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = store.Retrieve(query, 3)
		}
	})
	return scenarioFrom("Retrieve", n, dims, res), nil
}

func scenarioFrom(op string, n, dims int, res testing.BenchmarkResult) Scenario {
	ms := float64(res.NsPerOp()) / 1e6
	pct := (ms / ADR001LatencyThresholdMS) * 100
	crossed := ms > ADR001LatencyThresholdMS
	ci := n <= 1_000
	line := fmt.Sprintf("%s@%s×%d: %s — %s",
		op, formatN(n), dims, formatMS(ms), formatMargin(pct, crossed))
	return Scenario{
		Op:             op,
		N:              n,
		Dims:           dims,
		NsPerOp:        res.NsPerOp(),
		AllocsPerOp:    res.AllocsPerOp(),
		MsPerOp:        ms,
		ThresholdMs:    ADR001LatencyThresholdMS,
		PctOfThreshold: pct,
		Crossed:        crossed,
		CIGuard:        ci,
		HumanLine:      line,
		BenchmarkN:     res.N,
	}
}

func buildIndex(seed int64, n, dims int) (*vector.Index, []float32, error) {
	idx := vector.NewIndexWithDims(dims)
	for i := 0; i < n; i++ {
		emb := synthEmbedding(seed, dims, i)
		node := vector.Node{
			ID:           synthNodeID(i),
			DimensionDUA: "Bench",
			Difficulty:   "basico",
			Format:       "practica",
			ResourceURL:  fmt.Sprintf("bench://node/%d", i),
			Embedding:    emb,
		}
		if err := idx.Upsert(node); err != nil {
			return nil, nil, fmt.Errorf("upsert node %d: %w", i, err)
		}
	}
	query := synthEmbedding(seed, dims, n+7) // distinct from corpus ids
	return idx, query, nil
}

func buildStore(seed int64, n, dims int) (*rag.Store, []float32) {
	store := rag.NewStore()
	for i := 0; i < n; i++ {
		store.Upsert(rag.Chunk{
			ID:        fmt.Sprintf("bench-chunk-%d", i),
			Source:    "bench://corpus",
			Title:     fmt.Sprintf("chunk-%d", i),
			Text:      fmt.Sprintf("synthetic chunk %d", i),
			Embedding: synthEmbedding(seed, dims, i),
		})
	}
	return store, synthEmbedding(seed, dims, n+7)
}

// synthEmbedding is deterministic: rand.NewSource(seed^mix) — never unseeded.
func synthEmbedding(seed int64, dims, index int) []float32 {
	mix := seed ^ (int64(dims) * 1_000_003) ^ (int64(index) * 9176)
	r := rand.New(rand.NewSource(mix))
	v := make([]float32, dims)
	var sum float64
	for i := range v {
		x := float32(r.Float64()*2 - 1)
		v[i] = x
		sum += float64(x) * float64(x)
	}
	if sum == 0 {
		v[0] = 1
		return v
	}
	inv := float32(1 / math.Sqrt(sum))
	for i := range v {
		v[i] *= inv
	}
	return v
}

// synthNodeID builds a Crockford-valid ULID (26 chars) with zero wall clock.
func synthNodeID(i int) string {
	// 10-char time + 16-char entropy; alphabet is Crockford (hex subset OK).
	ulid := fmt.Sprintf("01J0000000%016X", i)
	return fmt.Sprintf("dua::Bench::basico::practica::%s", ulid)
}

func formatN(n int) string {
	switch {
	case n >= 1000 && n%1000 == 0:
		k := n / 1000
		if k >= 1000 && k%1000 == 0 {
			return fmt.Sprintf("%dM", k/1000)
		}
		return fmt.Sprintf("%dK", k)
	default:
		return fmt.Sprintf("%d", n)
	}
}

func formatMS(ms float64) string {
	if ms < 0.001 {
		return fmt.Sprintf("%.3fµs", ms*1000)
	}
	if ms < 1 {
		return fmt.Sprintf("%.3fms", ms)
	}
	return fmt.Sprintf("%.2fms", ms)
}

func formatMargin(pct float64, crossed bool) string {
	if crossed {
		return fmt.Sprintf("%.0f%% del umbral ADR-001 (CRUCE)", pct)
	}
	return fmt.Sprintf("%.0f%% del umbral ADR-001", pct)
}

func uniqueSorted(in []int) []int {
	seen := map[int]struct{}{}
	out := make([]int, 0, len(in))
	for _, v := range in {
		if v <= 0 {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Ints(out)
	return out
}

// WriteJSON persists the machine-readable report.
func WriteJSON(dir string, rep Report) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "bench.json")
	b, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// FormatTable writes the human-readable calibrate-style report.
func FormatTable(w io.Writer, rep Report) {
	fmt.Fprintf(w, "bench seed=%d threshold=%.0fms scenarios=%d\n\n",
		rep.Seed, rep.ThresholdMs, len(rep.Scenarios))
	for _, s := range rep.Scenarios {
		extra := ""
		if s.Crossed {
			extra = "  ← CRUCE"
		}
		fmt.Fprintf(w, "  %s  (ns/op=%d allocs/op=%d)%s\n",
			s.HumanLine, s.NsPerOp, s.AllocsPerOp, extra)
	}
	fmt.Fprintln(w)
	if rep.CIGuardFailed {
		fmt.Fprintln(w, "CI GUARD: al menos un escenario ≤1K cruzó el umbral ADR-001 (regresión algorítmica).")
	} else if rep.AnyCrossed {
		fmt.Fprintln(w, "Aviso: hay cruces de umbral (escala); escenarios ≤1K OK.")
	} else {
		fmt.Fprintln(w, "Ningún escenario cruzó el umbral ADR-001 de 5 ms/consulta.")
	}
}
