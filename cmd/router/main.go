package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	vectorv1 "github.com/vectorial-dua/avlp/gen/avlp/vector/v1"
	"github.com/vectorial-dua/avlp/pkg/livestation"
	"github.com/vectorial-dua/avlp/pkg/rag"
	"github.com/vectorial-dua/avlp/pkg/vector"
)

const defaultAddr = ":50051"

type server struct {
	vectorv1.UnimplementedVectorRouterServer
	router *vector.Router
}

type liveBridge struct {
	gen *livestation.Generator
}

func (b liveBridge) GenerateLive(ctx context.Context, req vector.LiveRequest) (vector.LiveResult, error) {
	res, err := b.gen.Generate(ctx, livestation.Request{
		StudentID:      req.StudentID,
		DoubtText:      req.DoubtText,
		QueryEmbedding: req.QueryEmbedding,
		Frustration:    req.Frustration,
		Dimension:      req.Dimension,
		Format:         req.Format,
		TrackingULID:   req.TrackingULID,
	})
	if err != nil {
		return vector.LiveResult{}, err
	}
	return vector.LiveResult{
		Node:         res.Node,
		Content:      res.Content,
		Sources:      res.Sources,
		TrackingULID: res.TrackingULID,
	}, nil
}

func (s *server) QueryNearestNode(ctx context.Context, req *vectorv1.VectorQuery) (*vectorv1.RouteResult, error) {
	studentID := ""
	frustration := float32(0.5)
	format := ""
	if st := req.GetStudentState(); st != nil {
		studentID = st.GetStudentId()
		if sess := st.GetSession(); sess != nil {
			frustration = sess.GetFrustrationSignal()
			format = sess.GetPreferredFormat()
		}
		dims := st.GetDimensions()
		if frustration == 0 && len(dims) >= 3 {
			frustration = dims[2]
		}
	}

	outcome, err := s.router.QueryNearestWithOptions(ctx, studentID, req.GetQueryEmbedding(), req.GetMinSimilarityThreshold(), vector.QueryOptions{
		DoubtText:   req.GetQueryText(),
		Frustration: frustration,
		Dimension:   "Representacion",
		Format:      format,
	})
	if err != nil {
		return nil, err
	}

	if outcome.Matched {
		return &vectorv1.RouteResult{
			Outcome: &vectorv1.RouteResult_Matched{
				Matched: &vectorv1.NodeResponse{
					NodeId:            outcome.Node.ID,
					DimensionDua:      outcome.Node.DimensionDUA,
					ResourceUrl:       outcome.Node.ResourceURL,
					SimilarityScore:   outcome.Similarity,
					IsLiveGenerated:   outcome.IsLiveGenerated,
					RetrievedSources:  outcome.RetrievedSources,
					LiveContent:       outcome.LiveContent,
				},
			},
		}, nil
	}

	return &vectorv1.RouteResult{
		Outcome: &vectorv1.RouteResult_Pending{
			Pending: &vectorv1.LiveStationPending{
				TrackingUlid: outcome.TrackingULID,
				Status:       outcome.LiveStatus,
				Message:      outcome.LiveMessage,
			},
		},
	}, nil
}

func main() {
	addr := defaultAddr
	if v := os.Getenv("AVLP_ROUTER_ADDR"); v != "" {
		addr = v
	}

	index := vector.NewIndex()
	if err := vector.SeedDemoNodes(index); err != nil {
		log.Fatalf("seed demo nodes: %v", err)
	}

	bus := vector.NewEventBus()
	bus.Subscribe(func(evt vector.NodeNotFoundEvent) {
		log.Printf("NodeNotFoundEvent student=%s tracking=%s best=%.4f threshold=%.2f",
			evt.StudentID, evt.TrackingULID, evt.BestSimilarity, evt.Threshold)
	})

	router := vector.NewRouter(index, bus)

	kb := os.Getenv("AVLP_KB_ROOT")
	if kb == "" {
		kb = "data/knowledge_base"
	}
	if abs, err := filepath.Abs(kb); err == nil {
		kb = abs
	}

	if router.Enabled {
		store := rag.NewStore()
		emb := rag.DefaultEmbedder()
		n, err := rag.IngestWalk(context.Background(), store, rag.IngestOptions{Root: kb, Embedder: emb})
		if err != nil {
			log.Printf("RAG ingest warning: %v (miss path will stay pending)", err)
		} else {
			log.Printf("RAG knowledge base indexed: %d chunks from %s", n, kb)
			gen := &livestation.Generator{
				Retriever: rag.NewRetriever(store, emb, 3),
				Nodes:     index,
			}
			router.Live = liveBridge{gen: gen}
		}
	} else {
		log.Printf("RAG disabled (AVLP_RAG_ENABLED=false)")
	}

	srv := grpc.NewServer()
	vectorv1.RegisterVectorRouterServer(srv, &server{router: router})
	reflection.Register(srv)

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("listen %s: %v", addr, err)
	}

	go func() {
		log.Printf("AVLP vector router listening on %s (%d nodes indexed)", addr, index.Len())
		if err := srv.Serve(lis); err != nil {
			log.Fatalf("serve: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("shutting down router...")
	stopped := make(chan struct{})
	go func() {
		srv.GracefulStop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		srv.Stop()
	}
}
