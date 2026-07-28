package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	vectorv1 "github.com/vectorial-dua/avlp/gen/avlp/vector/v1"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:50051", "router gRPC address")
	concurrency := flag.Int("c", 64, "concurrent clients (match|miss)")
	requests := flag.Int("n", 1000, "total requests (match|miss)")
	mode := flag.String("mode", "match", "match | miss | poll")
	student := flag.String("student", "demo-student", "student_id for poll mode")
	pollEvery := flag.Duration("poll-every", 400*time.Millisecond, "GetLiveStation interval (poll)")
	pollTimeout := flag.Duration("poll-timeout", 15*time.Second, "max wait for ready/failed (poll)")
	flag.Parse()

	conn, err := grpc.NewClient(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	client := vectorv1.NewVectorRouterClient(conn)

	switch strings.ToLower(*mode) {
	case "poll":
		if err := runPollFlow(client, *student, *pollEvery, *pollTimeout); err != nil {
			log.Fatal(err)
		}
	case "match", "miss":
		runLoad(client, *mode, *concurrency, *requests)
	default:
		fmt.Fprintf(os.Stderr, "unknown mode %q (want match|miss|poll)\n", *mode)
		os.Exit(2)
	}
}

func runPollFlow(client vectorv1.VectorRouterClient, studentID string, every, timeout time.Duration) error {
	// Distant embedding → miss path → LiveStationPending + tracking_ulid.
	query := []float32{0.01, 0.02, 0.03, 0.99, 0.01}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	route, err := client.QueryNearestNode(ctx, &vectorv1.VectorQuery{
		StudentState: &vectorv1.StudentVector{
			StudentId:  studentID,
			Dimensions: []float32{0.5, 0.5, 0.4, 0.6, 0.5},
			Timestamp:  time.Now().UnixMilli(),
		},
		QueryEmbedding:         query,
		MinSimilarityThreshold: 0.85,
		QueryText:              "duda sin nodo cercano en el índice",
	})
	if err != nil {
		return fmt.Errorf("QueryNearestNode: %w", err)
	}

	if m := route.GetMatched(); m != nil {
		fmt.Printf("matched immediately node_id=%s live=%v\n", m.GetNodeId(), m.GetIsLiveGenerated())
		if m.GetIsLiveGenerated() {
			fmt.Printf("live_content_preview=%q\n", truncate(m.GetLiveContent(), 120))
		}
		return nil
	}

	pending := route.GetPending()
	if pending == nil {
		return fmt.Errorf("expected matched or pending outcome")
	}
	fmt.Printf("pending tracking_ulid=%s status=%s\nmessage=%s\n",
		pending.GetTrackingUlid(), pending.GetStatus(), pending.GetMessage())

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(every)
		pctx, pcancel := context.WithTimeout(context.Background(), 3*time.Second)
		st, err := client.GetLiveStation(pctx, &vectorv1.LiveStationQuery{
			TrackingUlid: pending.GetTrackingUlid(),
			StudentId:    studentID,
		})
		pcancel()
		if err != nil {
			if s, ok := status.FromError(err); ok {
				fmt.Printf("GetLiveStation: %s — %s\n", s.Code(), s.Message())
			} else {
				fmt.Printf("GetLiveStation: %v\n", err)
			}
			continue
		}
		fmt.Printf("poll status=%s message=%s\n", st.GetStatus(), st.GetStudentMessage())
		switch st.GetStatus() {
		case "ready":
			fmt.Printf("ready node_id=%s sources=%v\ncontent_preview=%q\n",
				st.GetNodeId(), st.GetRetrievedSources(), truncate(st.GetLiveContent(), 160))
			return nil
		case "failed":
			return nil
		}
	}
	return fmt.Errorf("timeout waiting for station %s", pending.GetTrackingUlid())
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func runLoad(client vectorv1.VectorRouterClient, mode string, concurrency, requests int) {
	query := []float32{0.90, 0.12, 0.08, 0.22, 0.18} // near env-diagram seed
	if mode == "miss" {
		query = []float32{0.01, 0.02, 0.03, 0.99, 0.01} // distant from seeds
	}

	var (
		wg       sync.WaitGroup
		okCount  atomic.Int64
		errCount atomic.Int64
		latSum   atomic.Int64
		latMax   atomic.Int64
	)

	jobs := make(chan int, requests)
	for i := 0; i < requests; i++ {
		jobs <- i
	}
	close(jobs)

	start := time.Now()
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := range jobs {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				reqStart := time.Now()
				res, err := client.QueryNearestNode(ctx, &vectorv1.VectorQuery{
					StudentState: &vectorv1.StudentVector{
						StudentId:  fmt.Sprintf("student-%d-%d", worker, i),
						Dimensions: []float32{0.5, 0.5, 0.4, 0.6, 0.5},
						Timestamp:  time.Now().UnixMilli(),
					},
					QueryEmbedding:         query,
					MinSimilarityThreshold: 0.85,
				})
				cancel()
				elapsed := time.Since(reqStart).Microseconds()
				latSum.Add(elapsed)
				for {
					cur := latMax.Load()
					if elapsed <= cur || latMax.CompareAndSwap(cur, elapsed) {
						break
					}
				}
				if err != nil {
					errCount.Add(1)
					continue
				}
				okCount.Add(1)
				_ = res
			}
		}(w)
	}
	wg.Wait()
	total := time.Since(start)

	ok := okCount.Load()
	avgUs := int64(0)
	if ok > 0 {
		avgUs = latSum.Load() / ok
	}
	fmt.Printf("requests=%d ok=%d err=%d concurrency=%d wall=%s avg_latency=%dus max_latency=%dus\n",
		requests, ok, errCount.Load(), concurrency, total, avgUs, latMax.Load())
}
