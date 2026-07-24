package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	vectorv1 "github.com/vectorial-dua/avlp/gen/avlp/vector/v1"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:50051", "router gRPC address")
	concurrency := flag.Int("c", 64, "concurrent clients")
	requests := flag.Int("n", 1000, "total requests")
	mode := flag.String("mode", "match", "match | miss")
	flag.Parse()

	conn, err := grpc.NewClient(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	client := vectorv1.NewVectorRouterClient(conn)

	query := []float32{0.90, 0.12, 0.08, 0.22, 0.18} // near env-diagram seed
	if *mode == "miss" {
		query = []float32{0.01, 0.02, 0.03, 0.99, 0.01} // distant from seeds
	}

	var (
		wg      sync.WaitGroup
		okCount atomic.Int64
		errCount atomic.Int64
		latSum  atomic.Int64
		latMax  atomic.Int64
	)

	jobs := make(chan int, *requests)
	for i := 0; i < *requests; i++ {
		jobs <- i
	}
	close(jobs)

	start := time.Now()
	for w := 0; w < *concurrency; w++ {
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
					QueryEmbedding:          query,
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
		*requests, ok, errCount.Load(), *concurrency, total, avgUs, latMax.Load())
}
