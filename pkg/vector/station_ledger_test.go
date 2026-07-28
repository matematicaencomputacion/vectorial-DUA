package vector_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vectorial-dua/avlp/pkg/vector"
)

type stubLive struct {
	mu    sync.Mutex
	fail  bool
	calls atomic.Int32
}

func (s *stubLive) GenerateLive(ctx context.Context, req vector.LiveRequest) (vector.LiveResult, error) {
	_ = ctx
	s.calls.Add(1)
	s.mu.Lock()
	fail := s.fail
	s.mu.Unlock()
	if fail {
		return vector.LiveResult{}, errors.New("rag unavailable")
	}
	return vector.LiveResult{
		Node: vector.Node{
			ID:           "dua::Representacion::adaptativo::conceptual::01TESTLIVE0000000000000000",
			DimensionDUA: "Representacion",
			Format:       "conceptual",
			Embedding:    make([]float32, vector.ContentEmbedDims),
		},
		Content:      "live content for " + req.DoubtText,
		Sources:      []string{"kb/demo.md"},
		TrackingULID: req.TrackingULID,
	}, nil
}

func (s *stubLive) setFail(v bool) {
	s.mu.Lock()
	s.fail = v
	s.mu.Unlock()
}

func novelQuery(dims int) []float32 {
	q := make([]float32, dims)
	q[dims-1] = 1
	return q
}

func TestStationLedgerRegistersOnMiss(t *testing.T) {
	idx := vector.NewIndex()
	r := vector.NewRouter(idx, vector.NewEventBus())
	r.Enabled = false

	out, err := r.QueryNearestWithOptions(context.Background(), "stu-1", novelQuery(idx.Dims()), 0.85, vector.QueryOptions{
		DoubtText: "duda inédita de calibración",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Matched || out.TrackingULID == "" {
		t.Fatalf("expected pending, got %+v", out)
	}
	rec := r.Ledger.Get(out.TrackingULID)
	if rec == nil || rec.Status != vector.StationInProgress {
		t.Fatalf("expected in_progress ledger entry, got %+v", rec)
	}
	if rec.StudentID != "stu-1" || rec.Request.DoubtText == "" {
		t.Fatalf("request not retained: %+v", rec)
	}
}

func TestStationLedgerMarksReadyOnLiveSuccess(t *testing.T) {
	idx := vector.NewIndex()
	r := vector.NewRouter(idx, vector.NewEventBus())
	r.Enabled = true
	r.Live = &stubLive{}

	out, err := r.QueryNearestWithOptions(context.Background(), "stu-2", novelQuery(idx.Dims()), 0.85, vector.QueryOptions{
		DoubtText: "necesito una estación",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !out.Matched || !out.IsLiveGenerated {
		t.Fatalf("expected live match, got %+v", out)
	}
	rec := r.Ledger.Get(out.TrackingULID)
	if rec == nil || rec.Status != vector.StationReady || rec.Result == nil {
		t.Fatalf("expected ready, got %+v", rec)
	}
}

func TestStationLedgerFailedThenLazyRetrySucceeds(t *testing.T) {
	idx := vector.NewIndex()
	r := vector.NewRouter(idx, vector.NewEventBus())
	r.Enabled = true
	live := &stubLive{fail: true}
	r.Live = live

	out, err := r.QueryNearestWithOptions(context.Background(), "stu-3", novelQuery(idx.Dims()), 0.85, vector.QueryOptions{
		DoubtText: "bloqueo con RAG caído",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Matched {
		t.Fatal("expected pending after live failure")
	}
	rec := r.Ledger.Get(out.TrackingULID)
	if rec == nil || rec.Status != vector.StationFailed {
		t.Fatalf("expected failed, got %+v", rec)
	}

	live.setFail(false)
	got, err := r.LookupStation(context.Background(), out.TrackingULID, "stu-3")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Status != vector.StationReady || got.Result == nil {
		t.Fatalf("expected ready after retry, got %+v", got)
	}
	if live.calls.Load() < 2 {
		t.Fatalf("expected retry call, calls=%d", live.calls.Load())
	}
}

func TestStationLedgerTTLExpiresOnAccess(t *testing.T) {
	ledger := vector.NewStationLedger(50 * time.Millisecond)
	ledger.RegisterInProgress("ulid-ttl", "stu", vector.LiveRequest{DoubtText: "x"})
	if ledger.Get("ulid-ttl") == nil {
		t.Fatal("expected record")
	}
	time.Sleep(80 * time.Millisecond)
	if ledger.Get("ulid-ttl") != nil {
		t.Fatal("expected TTL purge")
	}
}

func TestStationLedgerLookupHidesWrongStudent(t *testing.T) {
	idx := vector.NewIndex()
	r := vector.NewRouter(idx, vector.NewEventBus())
	r.Enabled = false
	out, err := r.QueryNearest("stu-owner", novelQuery(idx.Dims()), 0.85)
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.LookupStation(context.Background(), out.TrackingULID, "stu-other")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatal("wrong student must not see station")
	}
}

func TestStationLedgerConcurrentLookupSingleGenerate(t *testing.T) {
	idx := vector.NewIndex()
	r := vector.NewRouter(idx, vector.NewEventBus())
	r.Enabled = false
	out, err := r.QueryNearestWithOptions(context.Background(), "stu-race", novelQuery(idx.Dims()), 0.85, vector.QueryOptions{
		DoubtText: "race",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Matched {
		t.Fatal("expected pending")
	}

	live := &registeringLive{idx: idx, delay: 80 * time.Millisecond}
	r.Enabled = true
	r.Live = live
	before := idx.Len()

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = r.LookupStation(context.Background(), out.TrackingULID, "stu-race")
		}()
	}
	wg.Wait()

	if live.calls.Load() != 1 {
		t.Fatalf("expected exactly 1 GenerateLive, got %d", live.calls.Load())
	}
	if idx.Len() != before+1 {
		t.Fatalf("expected one new node, before=%d after=%d", before, idx.Len())
	}
	rec := r.Ledger.Get(out.TrackingULID)
	if rec == nil || rec.Status != vector.StationReady {
		t.Fatalf("expected ready, got %+v", rec)
	}
}

type registeringLive struct {
	idx   *vector.Index
	delay time.Duration
	calls atomic.Int32
}

func (s *registeringLive) GenerateLive(ctx context.Context, req vector.LiveRequest) (vector.LiveResult, error) {
	_ = ctx
	s.calls.Add(1)
	if s.delay > 0 {
		time.Sleep(s.delay)
	}
	emb := make([]float32, s.idx.Dims())
	emb[0] = 1
	node, err := s.idx.RegisterNode("Representacion", "adaptativo", "conceptual", "live://race", emb)
	if err != nil {
		return vector.LiveResult{}, err
	}
	return vector.LiveResult{
		Node:         node,
		Content:      "único",
		Sources:      []string{"kb/one.md"},
		TrackingULID: req.TrackingULID,
	}, nil
}
