package telemetry_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/vectorial-dua/avlp/harness/telemetry"
)

func TestSnapshotExport(t *testing.T) {
	c := telemetry.NewCollector()
	c.Inc("routing_match_total", 2)
	c.ObserveRouting(2 * time.Millisecond)
	c.TraceLLM(telemetry.LLMSpan{
		Model:    "stub",
		Purpose:  "eval_judge",
		Success:  true,
		LatencyMS: 5,
	})
	snap := c.Snapshot()
	if snap.Counters["routing_match_total"] != 2 {
		t.Fatalf("counters=%v", snap.Counters)
	}
	if len(snap.LLMSpans) != 1 {
		t.Fatalf("spans=%d", len(snap.LLMSpans))
	}
	path := filepath.Join(t.TempDir(), "snap.json")
	if err := telemetry.WriteJSON(path, snap); err != nil {
		t.Fatal(err)
	}
}
