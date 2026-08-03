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
	"github.com/vectorial-dua/avlp/pkg/knowledge"
	"github.com/vectorial-dua/avlp/pkg/livestation"
	"github.com/vectorial-dua/avlp/pkg/rag"
	"github.com/vectorial-dua/avlp/pkg/rogerian"
	"github.com/vectorial-dua/avlp/pkg/session"
	"github.com/vectorial-dua/avlp/pkg/vector"
)

const defaultAddr = "127.0.0.1:50051"

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

	synthesizer, err := rogerian.NewHTTPSynthesizerFromEnv()
	if err != nil {
		log.Fatalf("LLM synthesizer: %v", err)
	}
	if synthesizer == nil {
		log.Printf("LLM synthesizer disabled: extractive fallback active (set AVLP_LLM_URL to enable)")
	} else {
		log.Printf("LLM synthesizer active: model=%s", synthesizer.ModelName())
	}

	index := vector.NewIndexWithDims(emb.Dims())
	if err := vector.SeedDemoNodes(index, emb); err != nil {
		log.Fatalf("seed demo nodes: %v", err)
	}

	bus := vector.NewEventBus()
	bus.Subscribe(func(evt vector.NodeNotFoundEvent) {
		log.Printf("NodeNotFoundEvent student=%s tracking=%s best=%.4f threshold=%.2f",
			evt.StudentID, evt.TrackingULID, evt.BestSimilarity, evt.Threshold)
	})

	configPath := strings.TrimSpace(os.Getenv("AVLP_CONFIG_PATH"))
	if configPath == "" {
		configPath = vector.DefaultConfigPath
	}
	threshold := vector.ResolveEffectiveThreshold(configPath)
	switch threshold.Source {
	case vector.ThresholdSourceEnv:
		log.Printf("similarity threshold=%.3f source=env (AVLP_SIMILARITY_THRESHOLD)", threshold.Value)
	case vector.ThresholdSourceFile:
		log.Printf("similarity threshold=%.3f source=file path=%s", threshold.Value, threshold.ConfigPath)
	case vector.ThresholdSourceDefault:
		log.Printf("similarity threshold=%.3f source=default", threshold.Value)
	default:
		log.Printf("similarity threshold=%.3f source=unknown", threshold.Value)
	}

	router := vector.NewRouter(index, bus)
	router.DefaultThreshold = threshold.Value

	kb := os.Getenv("AVLP_KB_ROOT")
	if kb == "" {
		kb = "data/knowledge_base"
	}
	if abs, err := filepath.Abs(kb); err == nil {
		kb = abs
	}

	var store *rag.Store
	var liveGen *livestation.Generator
	if router.Enabled {
		store = rag.NewStore()
		n, err := rag.IngestWalk(ctx, store, rag.IngestOptions{Root: kb, Embedder: emb})
		if err != nil {
			log.Printf("RAG ingest warning: %v (miss path will stay pending)", err)
		} else {
			log.Printf("RAG knowledge base indexed: %d chunks from %s", n, kb)
			liveGen = &livestation.Generator{
				Retriever:   rag.NewRetriever(store, emb, 3),
				Nodes:       index,
				Synthesizer: synthesizer,
				Logf:        log.Printf,
			}
			router.Live = liveGen
		}
	} else {
		log.Printf("RAG disabled (AVLP_RAG_ENABLED=false)")
	}

	profiles, profileCloser := openProfileStore()
	interactions := dua.NewInteractionStoreWithProfiles(profiles)
	interactions.Logf = log.Printf
	var reg *dua.Registry
	var mutator *dua.Mutator
	nodesDir := os.Getenv("AVLP_INTERACTIVE_NODES_DIR")
	if nodesDir == "" {
		nodesDir = "data/nodes/interactive"
	}
	if abs, err := filepath.Abs(nodesDir); err == nil {
		nodesDir = abs
	}

	if dua.EnabledFromEnv() {
		reg = dua.NewRegistry()
		reg.Logf = log.Printf
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
					Concepts:     append([]string(nil), node.Concepts...),
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
	if liveGen != nil && reg != nil {
		liveGen.AvailableTopics = reg.TopicTitles()
	}

	kgPath := strings.TrimSpace(os.Getenv("AVLP_KNOWLEDGE_GRAPH_PATH"))
	if kgPath == "" {
		kgPath = "data/knowledge/curriculum.json"
	}
	if abs, err := filepath.Abs(kgPath); err == nil {
		kgPath = abs
	}
	binder := &knowledge.IndexBinder{Index: index, Registry: reg}
	kg, _, err := knowledge.LoadFile(kgPath, knowledge.LoadOptions{
		Strict: knowledge.StrictFromEnv(),
		Logf:   log.Printf,
		Binder: binder,
	})
	if err != nil {
		log.Fatalf("knowledge graph: %v", err)
	}
	unbound := binder.UnboundResourceCount()
	log.Printf("grafo: %d conceptos, %d aristas, %d recursos sin concepto",
		len(kg.ConceptIDs()), len(kg.Edges()), unbound)

	visits, visitCloser := openConceptVisitStore()
	advisor := &knowledge.Advisor{Graph: kg, Visits: visits, Logf: log.Printf}

	srvImpl := routerserver.New(routerserver.Deps{
		Router:        router,
		Registry:      reg,
		Mutator:       mutator,
		QueryEmbedder: emb,
		Profiles:      profiles,
		Interactions:  interactions,
		Graph:         kg,
		Visits:        visits,
		Advisor:       advisor,
		Promoter: &dua.LiveStationPromoter{
			Ledger:   router.Ledger,
			Index:    index,
			Registry: reg,
			SeedsDir: nodesDir,
		},
	})

	srv := grpc.NewServer()
	vectorv1.RegisterVectorRouterServer(srv, srvImpl)
	reflection.Register(srv)

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("listen %s: %v", addr, err)
	}

	go func() {
		if session.SecureModeFromEnv() {
			log.Printf("session auth: secure mode active (AVLP_SESSION_SECRET set)")
			if !isLoopbackListenAddr(addr) {
				log.Printf("WARNING: el router acepta identidad por metadata; exponerlo fuera de loopback sin mTLS permite suplantación (AVLP_ROUTER_ADDR=%s)", addr)
			}
		} else {
			log.Printf("session auth: open mode (AVLP_SESSION_SECRET empty) — metadata optional")
		}
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
	if visitCloser != nil {
		if err := visitCloser.Close(); err != nil {
			log.Printf("concept visit store close: %v", err)
		} else {
			log.Printf("concept visit store flushed")
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

func openConceptVisitStore() (knowledge.ConceptVisitStore, interface{ Close() error }) {
	path := strings.TrimSpace(os.Getenv("AVLP_CONCEPT_STORE_PATH"))
	if path == "" {
		log.Printf("concept visit store: in-memory (set AVLP_CONCEPT_STORE_PATH to persist)")
		return knowledge.NewMemoryConceptVisitStore(), nil
	}
	store, err := knowledge.NewFileConceptVisitStore(path)
	if err != nil {
		log.Fatalf("concept visit store: %v", err)
	}
	store.Logf = log.Printf
	log.Printf("concept visit store: file snapshot at %s", path)
	return store, store
}

func formatFromNodeID(nodeID string) string {
	parts, err := vector.ParseNodeID(nodeID)
	if err != nil || parts.Format == "" {
		return "visual"
	}
	return parts.Format
}

// isLoopbackListenAddr reports whether addr is a TCP listen target bound only to
// loopback (127.0.0.0/8, ::1, or hostname "localhost"). Empty host (":port")
// and non-loopback IPs are not loopback — the router trusts gRPC metadata for
// identity, so exposing it beyond loopback without mTLS enables impersonation.
func isLoopbackListenAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if host == "" {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return strings.EqualFold(host, "localhost")
}
