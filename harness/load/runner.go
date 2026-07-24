package load

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	vectorv1 "github.com/vectorial-dua/avlp/gen/avlp/vector/v1"
	"github.com/vectorial-dua/avlp/harness/telemetry"
)

// Config configures a gRPC load run.
type Config struct {
	Addr        string
	Concurrency int
	Requests    int
	Mode        string // match | miss
	Timeout     time.Duration
}

// Report summarizes load harness results.
type Report struct {
	RunID         string                 `json:"run_id"`
	Concurrency   int                    `json:"concurrency"`
	TotalRequests int                    `json:"total_requests"`
	OKRequests    int                    `json:"ok_requests"`
	ErrRequests   int                    `json:"err_requests"`
	ErrorRate     float64                `json:"error_rate"`
	QPS           float64                `json:"qps"`
	Latency       telemetry.LatencyStats `json:"latency"`
	WallMS        int64                  `json:"wall_ms"`
	SLOPass       bool                   `json:"slo_pass"`
	Message       string                 `json:"message,omitempty"`
}

// Runner executes concurrent QueryNearestNode calls.
type Runner struct {
	Tel *telemetry.Collector
}

// Run dials the router and executes the load profile.
func (r *Runner) Run(cfg Config) (Report, error) {
	if cfg.Addr == "" {
		cfg.Addr = "127.0.0.1:50051"
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 32
	}
	if cfg.Requests <= 0 {
		cfg.Requests = 500
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 2 * time.Second
	}

	conn, err := grpc.NewClient(cfg.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return Report{}, fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()
	client := vectorv1.NewVectorRouterClient(conn)

	query := []float32{0.90, 0.12, 0.08, 0.22, 0.18}
	if cfg.Mode == "miss" {
		query = []float32{0.01, 0.02, 0.03, 0.99, 0.01}
	}

	var (
		wg       sync.WaitGroup
		okCount  atomic.Int64
		errCount atomic.Int64
		mu       sync.Mutex
		samples  []float64
	)

	jobs := make(chan int, cfg.Requests)
	for i := 0; i < cfg.Requests; i++ {
		jobs <- i
	}
	close(jobs)

	start := time.Now()
	for w := 0; w < cfg.Concurrency; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := range jobs {
				ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
				t0 := time.Now()
				_, err := client.QueryNearestNode(ctx, &vectorv1.VectorQuery{
					StudentState: &vectorv1.StudentVector{
						StudentId:  fmt.Sprintf("load-%d-%d", worker, i),
						Dimensions: []float32{0.5, 0.5, 0.4, 0.6, 0.5},
						Timestamp:  time.Now().UnixMilli(),
					},
					QueryEmbedding:         query,
					MinSimilarityThreshold: 0.85,
				})
				elapsed := time.Since(t0)
				cancel()
				mu.Lock()
				samples = append(samples, float64(elapsed.Microseconds()))
				mu.Unlock()
				if r.Tel != nil {
					r.Tel.ObserveRouting(elapsed)
				}
				if err != nil {
					errCount.Add(1)
					continue
				}
				okCount.Add(1)
			}
		}(w)
	}
	wg.Wait()
	wall := time.Since(start)

	ok := int(okCount.Load())
	errs := int(errCount.Load())
	total := cfg.Requests
	errRate := 0.0
	if total > 0 {
		errRate = float64(errs) / float64(total)
	}
	qps := 0.0
	if wall.Seconds() > 0 {
		qps = float64(total) / wall.Seconds()
	}

	lat := microsToStats(samples)
	rep := Report{
		RunID:         start.UTC().Format("20060102T150405Z"),
		Concurrency:   cfg.Concurrency,
		TotalRequests: total,
		OKRequests:    ok,
		ErrRequests:   errs,
		ErrorRate:     errRate,
		QPS:           qps,
		Latency:       lat,
		WallMS:        wall.Milliseconds(),
		SLOPass:       errRate < 0.01,
	}
	if !rep.SLOPass {
		rep.Message = fmt.Sprintf("error_rate %.4f >= 0.01", errRate)
	}
	if r.Tel != nil {
		r.Tel.Inc("load_requests_total", int64(total))
		r.Tel.Inc("load_errors_total", int64(errs))
	}
	return rep, nil
}

func microsToStats(samples []float64) telemetry.LatencyStats {
	if len(samples) == 0 {
		return telemetry.LatencyStats{}
	}
	sorted := append([]float64(nil), samples...)
	sort.Float64s(sorted)
	toMS := func(us float64) float64 { return us / 1000.0 }
	pct := func(p float64) float64 {
		idx := int(float64(len(sorted)-1) * p)
		return toMS(sorted[idx])
	}
	var sum float64
	for _, v := range sorted {
		sum += v
	}
	return telemetry.LatencyStats{
		P50MS:  pct(0.50),
		P95MS:  pct(0.95),
		P99MS:  pct(0.99),
		MaxMS:  toMS(sorted[len(sorted)-1]),
		MeanMS: toMS(sum / float64(len(sorted))),
	}
}
