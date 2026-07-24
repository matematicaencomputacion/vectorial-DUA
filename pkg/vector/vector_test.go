package vector_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vectorial-dua/avlp/pkg/vector"
)

func TestCosineSimilarityIdentical(t *testing.T) {
	a := []float32{1, 0, 0}
	if got := vector.CosineSimilarity(a, a); got < 0.999 {
		t.Fatalf("expected ~1, got %v", got)
	}
}

func TestCosineSimilarityOrthogonal(t *testing.T) {
	a := []float32{1, 0}
	b := []float32{0, 1}
	if got := vector.CosineSimilarity(a, b); got > 0.001 {
		t.Fatalf("expected ~0, got %v", got)
	}
}

func TestNodeIDFormatAndChronologicalOrder(t *testing.T) {
	id1, err := vector.NewNodeID("Representacion", "basico", "visual")
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)
	id2, err := vector.NewNodeID("Representacion", "basico", "visual")
	if err != nil {
		t.Fatal(err)
	}
	if !vector.ValidateNodeID(id1) || !vector.ValidateNodeID(id2) {
		t.Fatalf("ids should validate: %s %s", id1, id2)
	}
	t1, err := vector.ULIDTime(id1)
	if err != nil {
		t.Fatal(err)
	}
	t2, err := vector.ULIDTime(id2)
	if err != nil {
		t.Fatal(err)
	}
	if t2.Before(t1) {
		t.Fatalf("expected chronological ULID order: %v before %v", t1, t2)
	}
}

func TestIndexUniqueULIDRing(t *testing.T) {
	idx := vector.NewIndex()
	node, err := idx.RegisterNode("Accion", "basico", "practica", "ide://x", []float32{1, 0, 0, 0, 0})
	if err != nil {
		t.Fatal(err)
	}
	parts, err := vector.ParseNodeID(node.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !idx.HasULID(parts.ULID) {
		t.Fatal("ulid should be in ring")
	}
	dup := node
	dup.ID = "dua::Accion::basico::practica::" + parts.ULID
	// same full ID is upsert-ok; different path same ULID would only happen if we craft it —
	// craft a different hierarchy with same ULID:
	crafted := node
	crafted.ID = "dua::Compromiso::basico::conceptual::" + parts.ULID
	if err := idx.Upsert(crafted); err == nil {
		t.Fatal("expected duplicate ULID ring error")
	}
}

func TestRouterStaticMatchAboveThreshold(t *testing.T) {
	idx := vector.NewIndex()
	if err := vector.SeedDemoNodes(idx); err != nil {
		t.Fatal(err)
	}
	var emitted int32
	bus := vector.NewEventBus()
	bus.Subscribe(func(evt vector.NodeNotFoundEvent) {
		atomic.AddInt32(&emitted, 1)
	})
	r := vector.NewRouter(idx, bus)

	out, err := r.QueryNearest("stu-1", []float32{0.92, 0.10, 0.05, 0.20, 0.15}, 0.85)
	if err != nil {
		t.Fatal(err)
	}
	if !out.Matched {
		t.Fatalf("expected static match, got pending tracking=%s sim=%v", out.TrackingULID, out.Similarity)
	}
	if out.Similarity < 0.85 {
		t.Fatalf("similarity below threshold: %v", out.Similarity)
	}
	if out.Node.DimensionDUA == "" {
		t.Fatal("expected DUA dimension strategy")
	}
	if atomic.LoadInt32(&emitted) != 0 {
		t.Fatal("should not emit NodeNotFound on match")
	}
}

func TestRouterLiveStationOnMiss(t *testing.T) {
	idx := vector.NewIndex()
	if err := vector.SeedDemoNodes(idx); err != nil {
		t.Fatal(err)
	}
	var got vector.NodeNotFoundEvent
	var mu sync.Mutex
	bus := vector.NewEventBus()
	bus.Subscribe(func(evt vector.NodeNotFoundEvent) {
		mu.Lock()
		got = evt
		mu.Unlock()
	})
	r := vector.NewRouter(idx, bus)

	out, err := r.QueryNearest("stu-2", []float32{0.0, 0.0, 0.0, 0.0, 1.0}, 0.85)
	if err != nil {
		t.Fatal(err)
	}
	if out.Matched {
		t.Fatalf("expected miss/live station, matched %s sim=%v", out.Node.ID, out.Similarity)
	}
	if out.TrackingULID == "" || out.LiveStatus != "in_progress" {
		t.Fatalf("expected in_progress tracking token, got %+v", out)
	}
	mu.Lock()
	defer mu.Unlock()
	if got.TrackingULID == "" || got.StudentID != "stu-2" {
		t.Fatalf("expected NodeNotFoundEvent, got %+v", got)
	}
	if got.Threshold != 0.85 {
		t.Fatalf("unexpected threshold %v", got.Threshold)
	}
}

func TestRouterInProcessLatencyP99(t *testing.T) {
	idx := vector.NewIndex()
	if err := vector.SeedDemoNodes(idx); err != nil {
		t.Fatal(err)
	}
	r := vector.NewRouter(idx, vector.NewEventBus())
	query := []float32{0.92, 0.10, 0.05, 0.20, 0.15}

	const n = 2000
	samples := make([]time.Duration, n)
	// Warmup
	for i := 0; i < 100; i++ {
		if _, err := r.QueryNearest("warm", query, 0.85); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < n; i++ {
		start := time.Now()
		if _, err := r.QueryNearest("stu-lat", query, 0.85); err != nil {
			t.Fatal(err)
		}
		samples[i] = time.Since(start)
	}

	// selection sort partial for p99 (n is small)
	for i := 0; i < n; i++ {
		min := i
		for j := i + 1; j < n; j++ {
			if samples[j] < samples[min] {
				min = j
			}
		}
		samples[i], samples[min] = samples[min], samples[i]
	}
	p99 := samples[(n*99)/100]
	t.Logf("router in-process p99=%s (%d ns) max=%s", p99, p99.Nanoseconds(), samples[n-1])
	if p99 > 15*time.Millisecond {
		t.Fatalf("p99 latency %s exceeds 15ms target", p99)
	}
}

func BenchmarkCosineSimilarity(b *testing.B) {
	a := []float32{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0}
	c := []float32{1.0, 0.9, 0.8, 0.7, 0.6, 0.5, 0.4, 0.3, 0.2, 0.1}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = vector.CosineSimilarity(a, c)
	}
}

func TestCosineThroughputTarget(t *testing.T) {
	a := make([]float32, 32)
	c := make([]float32, 32)
	for i := range a {
		a[i] = float32(i) * 0.01
		c[i] = float32(32-i) * 0.01
	}
	const target = 100_000
	const iterations = 300_000
	start := time.Now()
	for i := 0; i < iterations; i++ {
		_ = vector.CosineSimilarity(a, c)
	}
	elapsed := time.Since(start)
	opsPerSec := float64(iterations) / elapsed.Seconds()
	t.Logf("cosine throughput: %.0f ops/sec", opsPerSec)
	if opsPerSec < target {
		t.Fatalf("expected > %d ops/sec, got %.0f", target, opsPerSec)
	}
}
