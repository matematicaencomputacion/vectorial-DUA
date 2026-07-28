package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/vectorial-dua/avlp/harness/evals"
	"github.com/vectorial-dua/avlp/harness/load"
	"github.com/vectorial-dua/avlp/harness/sandbox"
	"github.com/vectorial-dua/avlp/harness/telemetry"
	"github.com/vectorial-dua/avlp/pkg/livestation"
	"github.com/vectorial-dua/avlp/pkg/rag"
	"github.com/vectorial-dua/avlp/pkg/vector"
)

type harnessLiveBridge struct{ g *livestation.Generator }

func (b harnessLiveBridge) GenerateLive(ctx context.Context, req vector.LiveRequest) (vector.LiveResult, error) {
	res, err := b.g.Generate(ctx, livestation.Request{
		StudentID: req.StudentID, DoubtText: req.DoubtText, QueryEmbedding: req.QueryEmbedding,
		Frustration: req.Frustration, Dimension: req.Dimension, Format: req.Format, TrackingULID: req.TrackingULID,
	})
	if err != nil {
		return vector.LiveResult{}, err
	}
	return vector.LiveResult{Node: res.Node, Content: res.Content, Sources: res.Sources, TrackingULID: res.TrackingULID}, nil
}

func main() {
	suite := flag.String("suite", "evals", "evals | sandbox | load | all")
	casesPath := flag.String("cases", "harness/evals/cases/routing_golden.json", "golden evals dataset")
	outDir := flag.String("out", "harness/out", "output directory for reports")
	addr := flag.String("addr", "127.0.0.1:50051", "router address for load suite")
	concurrency := flag.Int("c", 32, "load concurrency")
	requests := flag.Int("n", 500, "load total requests")
	mode := flag.String("mode", "match", "load mode: match | miss")
	embedderMode := flag.String("embedder", "hash", "evals embedder: hash (CI default) | env (AVLP_EMBEDDING_URL)")
	flag.Parse()

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		log.Fatalf("out dir: %v", err)
	}

	tel := telemetry.NewCollector()
	var failed bool

	runEvals := func() {
		emb, err := evals.ResolveEmbedder(*embedderMode)
		if err != nil {
			log.Fatalf("embedder: %v", err)
		}
		if err := rag.EnsureEmbedderDims(context.Background(), emb); err != nil {
			log.Fatalf("embedder dims: %v", err)
		}
		log.Printf("evals embedder=%s dims=%d", *embedderMode, emb.Dims())

		idx := vector.NewIndexWithDims(emb.Dims())
		if err := vector.SeedDemoNodes(idx, emb); err != nil {
			log.Fatalf("seed: %v", err)
		}
		store := rag.NewStore()
		kb := "data/knowledge_base"
		if n, err := rag.IngestWalk(context.Background(), store, rag.IngestOptions{Root: kb, Embedder: emb}); err != nil {
			log.Printf("RAG ingest skip: %v", err)
		} else {
			log.Printf("RAG chunks indexed: %d", n)
		}
		router := vector.NewRouter(idx, vector.NewEventBus())
		router.Live = harnessLiveBridge{g: &livestation.Generator{
			Retriever: rag.NewRetriever(store, emb, 3),
			Nodes:     idx,
			Tel:       tel,
		}}
		cases, err := evals.LoadCases(*casesPath)
		if err != nil {
			log.Fatalf("load cases: %v", err)
		}
		report := (&evals.Runner{Router: router, Tel: tel, Embedder: emb}).Run(cases)
		path := filepath.Join(*outDir, "eval_report.json")
		if err := evals.WriteReport(path, report); err != nil {
			log.Fatalf("write eval report: %v", err)
		}
		fmt.Printf("evals: pass_rate=%.2f passed=%d failed=%d → %s\n",
			report.PassRate, report.PassedCases, report.FailedCases, path)
		if report.FailedCases > 0 {
			failed = true
		}
	}

	runSandbox := func() {
		ex := sandbox.Executor{Policy: sandbox.DefaultPolicy()}
		res, err := ex.Run(context.Background(), sandbox.Request{
			Runtime: "node",
			Source:  "console.log('avlp-sandbox-ok')",
		})
		if err != nil {
			log.Fatalf("sandbox: %v", err)
		}
		path := filepath.Join(*outDir, "sandbox_result.json")
		b, _ := json.MarshalIndent(res, "", "  ")
		_ = os.WriteFile(path, b, 0o644)
		fmt.Printf("sandbox: exit=%d violation=%q duration=%s → %s\n",
			res.ExitCode, res.Violation, res.Duration, path)
		if res.Violation != "" || res.ExitCode != 0 {
			// Fallback demo with python if node missing
			res2, err2 := ex.Run(context.Background(), sandbox.Request{
				Runtime: "python",
				Source:  "print('avlp-sandbox-ok')",
			})
			if err2 == nil && res2.ExitCode == 0 && res2.Violation == "" {
				_ = os.WriteFile(path, mustJSON(res2), 0o644)
				fmt.Printf("sandbox(python fallback): exit=%d → %s\n", res2.ExitCode, path)
				tel.Inc("sandbox_ok_total", 1)
				return
			}
			failed = true
			tel.Inc("sandbox_violation_total", 1)
			return
		}
		tel.Inc("sandbox_ok_total", 1)
	}

	runLoad := func() {
		rep, err := (&load.Runner{Tel: tel}).Run(load.Config{
			Addr:        *addr,
			Concurrency: *concurrency,
			Requests:    *requests,
			Mode:        *mode,
			Timeout:     2 * time.Second,
		})
		if err != nil {
			log.Fatalf("load: %v", err)
		}
		path := filepath.Join(*outDir, "load_report.json")
		_ = os.WriteFile(path, mustJSON(rep), 0o644)
		fmt.Printf("load: qps=%.1f err_rate=%.4f p99=%.3fms slo_pass=%v → %s\n",
			rep.QPS, rep.ErrorRate, rep.Latency.P99MS, rep.SLOPass, path)
		if !rep.SLOPass {
			failed = true
		}
	}

	switch *suite {
	case "evals":
		runEvals()
	case "sandbox":
		runSandbox()
	case "load":
		runLoad()
	case "all":
		runEvals()
		runSandbox()
		runLoad()
	default:
		log.Fatalf("unknown suite %q", *suite)
	}

	snap := tel.Snapshot()
	snapPath := filepath.Join(*outDir, "telemetry_snapshot.json")
	if err := telemetry.WriteJSON(snapPath, snap); err != nil {
		log.Fatalf("telemetry: %v", err)
	}
	fmt.Printf("telemetry → %s\n", snapPath)

	if failed {
		os.Exit(1)
	}
}

func mustJSON(v any) []byte {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(err)
	}
	return b
}
