package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	vectorv1 "github.com/vectorial-dua/avlp/gen/avlp/vector/v1"
	"github.com/vectorial-dua/avlp/internal/routerserver"
	"github.com/vectorial-dua/avlp/pkg/dua"
	"github.com/vectorial-dua/avlp/pkg/livestation"
	"github.com/vectorial-dua/avlp/pkg/rag"
	"github.com/vectorial-dua/avlp/pkg/vector"
)

const defaultAddr = ":50051"

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

func main() {
	addr := defaultAddr
	if v := os.Getenv("AVLP_ROUTER_ADDR"); v != "" {
		addr = v
	}

	emb, err := rag.DefaultEmbedderE()
	if err != nil {
		log.Fatalf("embedder: %v", err)
	}
	ctx := context.Background()
	if err := rag.EnsureEmbedderDims(ctx, emb); err != nil {
		log.Fatalf("embedding backend: %v", err)
	}
	log.Printf("embedder active: dims=%d", emb.Dims())

	index := vector.NewIndexWithDims(emb.Dims())
	if err := vector.SeedDemoNodes(index, emb); err != nil {
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

	var store *rag.Store
	if router.Enabled {
		store = rag.NewStore()
		n, err := rag.IngestWalk(ctx, store, rag.IngestOptions{Root: kb, Embedder: emb})
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

	profiles, profileCloser := openProfileStore()
	interactions := dua.NewInteractionStoreWithProfiles(profiles)
	interactions.Logf = log.Printf
	var reg *dua.Registry
	var mutator *dua.Mutator

	if dua.EnabledFromEnv() {
		reg = dua.NewRegistry()
		nodesDir := os.Getenv("AVLP_INTERACTIVE_NODES_DIR")
		if nodesDir == "" {
			nodesDir = "data/nodes/interactive"
		}
		if abs, err := filepath.Abs(nodesDir); err == nil {
			nodesDir = abs
		}
		n, err := reg.LoadDir(nodesDir)
		if err != nil {
			log.Printf("interactive nodes load warning: %v", err)
		} else {
			log.Printf("interactive nodes loaded: %d from %s", n, nodesDir)
			// Index embeddings into vector space for nearest routing.
			reg.ForEach(func(node *dua.InteractiveVideoNode) {
				if len(node.Embedding) == 0 {
					return
				}
				fitted, err := vector.FitIndexEmbedding(node.Embedding, index.Dims())
				if err != nil {
					log.Printf("index interactive node %s: %v", node.NodeID, err)
					return
				}
				if err := index.Upsert(vector.Node{
					ID:           node.NodeID,
					DimensionDUA: node.DimensionDUA,
					Difficulty:   "basico",
					Format:       formatFromNodeID(node.NodeID),
					ResourceURL:  "interactive://" + node.NodeID,
					Embedding:    fitted,
				}); err != nil {
					log.Printf("index interactive node %s: %v", node.NodeID, err)
				}
			})
		}
		if store != nil && emb != nil {
			mutator = &dua.Mutator{
				Registry:  reg,
				Retriever: rag.NewRetriever(store, emb, 3),
			}
		} else {
			mutator = &dua.Mutator{Registry: reg}
		}
	} else {
		log.Printf("interactive nodes disabled (AVLP_INTERACTIVE_NODES=false)")
	}

	srvImpl := routerserver.New(routerserver.Deps{
		Router:        router,
		Registry:      reg,
		Mutator:       mutator,
		QueryEmbedder: emb,
		Profiles:      profiles,
		Interactions:  interactions,
	})

	srv := grpc.NewServer()
	vectorv1.RegisterVectorRouterServer(srv, srvImpl)
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
	if profileCloser != nil {
		if err := profileCloser.Close(); err != nil {
			log.Printf("profile store close: %v", err)
		} else {
			log.Printf("profile store flushed")
		}
	}
}

func openProfileStore() (dua.ProfileRepository, interface{ Close() error }) {
	path := strings.TrimSpace(os.Getenv("AVLP_PROFILE_STORE_PATH"))
	if path == "" {
		log.Printf("profile store: in-memory (set AVLP_PROFILE_STORE_PATH to persist)")
		return dua.NewProfileStore(), nil
	}
	store, err := dua.NewFileProfileStore(path)
	if err != nil {
		log.Fatalf("profile store: %v", err)
	}
	store.Logf = log.Printf
	log.Printf("profile store: file snapshot at %s", path)
	return store, store
}

func formatFromNodeID(nodeID string) string {
	parts, err := vector.ParseNodeID(nodeID)
	if err != nil || parts.Format == "" {
		return "visual"
	}
	return parts.Format
}
