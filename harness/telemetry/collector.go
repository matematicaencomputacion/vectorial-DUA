package telemetry

import (
	"crypto/rand"
	"encoding/json"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/oklog/ulid/v2"
)

// LLMSpan records one model invocation for harness observability.
type LLMSpan struct {
	SpanID           string `json:"span_id"`
	ParentRunID      string `json:"parent_run_id"`
	Model            string `json:"model"`
	Purpose          string `json:"purpose"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	LatencyMS        int64  `json:"latency_ms"`
	Success          bool   `json:"success"`
	ErrorMessage     string `json:"error_message,omitempty"`
	TimestampUnixMS  int64  `json:"timestamp_unix_ms"`
}

// LatencyStats holds percentile samples in milliseconds.
type LatencyStats struct {
	P50MS  float64 `json:"p50_ms"`
	P95MS  float64 `json:"p95_ms"`
	P99MS  float64 `json:"p99_ms"`
	MaxMS  float64 `json:"max_ms"`
	MeanMS float64 `json:"mean_ms"`
}

// Snapshot is the exportable control-plane telemetry document.
type Snapshot struct {
	SnapshotID      string            `json:"snapshot_id"`
	TimestampUnixMS int64             `json:"timestamp_unix_ms"`
	Counters        map[string]int64  `json:"counters"`
	RoutingLatency  LatencyStats      `json:"routing_latency"`
	LLMSpans        []LLMSpan         `json:"llm_spans"`
	Extra           map[string]any    `json:"extra,omitempty"`
}

// Collector is a process-local metrics + LLM trace sink.
type Collector struct {
	mu            sync.Mutex
	counters      map[string]*atomic.Int64
	routingMicros []float64
	llmSpans      []LLMSpan
}

// NewCollector creates an empty telemetry collector.
func NewCollector() *Collector {
	return &Collector{counters: make(map[string]*atomic.Int64)}
}

func (c *Collector) counter(name string) *atomic.Int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	if v, ok := c.counters[name]; ok {
		return v
	}
	v := &atomic.Int64{}
	c.counters[name] = v
	return v
}

// Inc increments a named counter.
func (c *Collector) Inc(name string, delta int64) {
	c.counter(name).Add(delta)
}

// ObserveRouting records a routing latency sample in microseconds.
func (c *Collector) ObserveRouting(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.routingMicros = append(c.routingMicros, float64(d.Microseconds()))
}

// TraceLLM appends an LLM span.
func (c *Collector) TraceLLM(span LLMSpan) {
	if span.SpanID == "" {
		span.SpanID = newULID()
	}
	if span.TimestampUnixMS == 0 {
		span.TimestampUnixMS = time.Now().UTC().UnixMilli()
	}
	c.mu.Lock()
	c.llmSpans = append(c.llmSpans, span)
	c.mu.Unlock()

	c.Inc("llm_calls_total", 1)
	if span.Success {
		c.Inc("llm_calls_success_total", 1)
	} else {
		c.Inc("llm_calls_error_total", 1)
	}
}

// Snapshot exports current counters, latency percentiles and spans.
func (c *Collector) Snapshot() Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()

	counters := make(map[string]int64, len(c.counters))
	for k, v := range c.counters {
		counters[k] = v.Load()
	}
	spans := append([]LLMSpan(nil), c.llmSpans...)
	return Snapshot{
		SnapshotID:      newULID(),
		TimestampUnixMS: time.Now().UTC().UnixMilli(),
		Counters:        counters,
		RoutingLatency:  percentilesFromMicros(c.routingMicros),
		LLMSpans:        spans,
	}
}

// WriteJSON writes a snapshot to path.
func WriteJSON(path string, snap Snapshot) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(snap)
}

func percentilesFromMicros(samples []float64) LatencyStats {
	if len(samples) == 0 {
		return LatencyStats{}
	}
	sorted := append([]float64(nil), samples...)
	for i := 0; i < len(sorted); i++ {
		min := i
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j] < sorted[min] {
				min = j
			}
		}
		sorted[i], sorted[min] = sorted[min], sorted[i]
	}
	toMS := func(us float64) float64 { return us / 1000.0 }
	pct := func(p float64) float64 {
		idx := int(float64(len(sorted)-1) * p)
		return toMS(sorted[idx])
	}
	var sum float64
	for _, v := range sorted {
		sum += v
	}
	return LatencyStats{
		P50MS:  pct(0.50),
		P95MS:  pct(0.95),
		P99MS:  pct(0.99),
		MaxMS:  toMS(sorted[len(sorted)-1]),
		MeanMS: toMS(sum / float64(len(sorted))),
	}
}

func newULID() string {
	id, err := ulid.New(ulid.Timestamp(time.Now().UTC()), ulid.Monotonic(rand.Reader, 0))
	if err != nil {
		return time.Now().UTC().Format("20060102150405.000")
	}
	return id.String()
}
