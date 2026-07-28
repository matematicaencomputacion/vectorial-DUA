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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	vectorv1 "github.com/vectorial-dua/avlp/gen/avlp/vector/v1"
	"github.com/vectorial-dua/avlp/pkg/dua"
	"github.com/vectorial-dua/avlp/pkg/livestation"
	"github.com/vectorial-dua/avlp/pkg/rag"
	"github.com/vectorial-dua/avlp/pkg/vector"
)

const defaultAddr = ":50051"

type server struct {
	vectorv1.UnimplementedVectorRouterServer
	router       *vector.Router
	reg          *dua.Registry
	mutator      *dua.Mutator
	profiles     *dua.ProfileStore
	interactions *dua.InteractionStore
}

const ackRecorded = "recorded"

func neutralAck() *vectorv1.Ack {
	return &vectorv1.Ack{Ok: true, Message: ackRecorded}
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
		hasInteractive := false
		if s.reg != nil {
			_, hasInteractive = s.reg.Get(outcome.Node.ID)
		}
		return &vectorv1.RouteResult{
			Outcome: &vectorv1.RouteResult_Matched{
				Matched: &vectorv1.NodeResponse{
					NodeId:                outcome.Node.ID,
					DimensionDua:          outcome.Node.DimensionDUA,
					ResourceUrl:           outcome.Node.ResourceURL,
					SimilarityScore:       outcome.Similarity,
					IsLiveGenerated:       outcome.IsLiveGenerated,
					RetrievedSources:      outcome.RetrievedSources,
					LiveContent:           outcome.LiveContent,
					HasInteractivePayload: hasInteractive,
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

func (s *server) GetInteractiveNode(ctx context.Context, req *vectorv1.NodeIdRequest) (*vectorv1.InteractiveVideoNode, error) {
	_ = ctx
	if s.reg == nil {
		return nil, status.Error(codes.FailedPrecondition, "interactive nodes disabled")
	}
	n, ok := s.reg.Get(req.GetNodeId())
	if !ok {
		return nil, status.Errorf(codes.NotFound, "interactive node %q not found", req.GetNodeId())
	}
	return dua.ToProto(n), nil
}

func (s *server) MutateInteractiveNode(ctx context.Context, req *vectorv1.MutateInteractiveRequest) (*vectorv1.MutateInteractiveResponse, error) {
	if s.mutator == nil {
		return nil, status.Error(codes.FailedPrecondition, "interactive mutation disabled")
	}
	res, err := s.mutator.Mutate(ctx, dua.MutateRequest{
		NodeID:         req.GetNodeId(),
		StudentID:      req.GetStudentId(),
		DoubtText:      req.GetDoubtText(),
		QueryEmbedding: req.GetQueryEmbedding(),
		Frustration:    req.GetFrustration(),
	})
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	return &vectorv1.MutateInteractiveResponse{
		Button: dua.ButtonToProto(res.Button),
		Node:   dua.ToProto(res.Node),
	}, nil
}

func (s *server) RecordBotoneraInteraction(ctx context.Context, req *vectorv1.BotoneraInteraction) (*vectorv1.Ack, error) {
	_ = ctx
	if s.profiles == nil {
		return nil, status.Error(codes.FailedPrecondition, "profile store disabled")
	}
	if s.reg == nil {
		return nil, status.Error(codes.FailedPrecondition, "interactive nodes disabled")
	}
	if req.GetStudentId() == "" || req.GetNodeId() == "" || req.GetVariantId() == "" {
		return nil, status.Error(codes.InvalidArgument, "student_id, node_id, and variant_id are required")
	}

	n, ok := s.reg.Get(req.GetNodeId())
	if !ok {
		return nil, status.Errorf(codes.NotFound, "interactive node %q not found", req.GetNodeId())
	}
	if n.BotoneraSchema == nil {
		return nil, status.Errorf(codes.NotFound, "node %q has no botonera_schema", req.GetNodeId())
	}
	if !dua.HasBotoneraVariant(n.BotoneraSchema, req.GetVariantId(), req.GetFormatId()) {
		return nil, status.Errorf(codes.NotFound, "variant %q not found in botonera schema for node %q", req.GetVariantId(), req.GetNodeId())
	}

	delta := dua.ResolveBotoneraDelta(n.BotoneraSchema, req.GetVariantId(), req.GetPreferenceDelta())
	if len(delta) == 0 {
		return neutralAck(), nil
	}
	if _, err := s.profiles.Apply(req.GetStudentId(), delta); err != nil {
		log.Printf("botonera profile delta skipped student=%s node=%s variant=%s: %v",
			req.GetStudentId(), req.GetNodeId(), req.GetVariantId(), err)
	}
	return neutralAck(), nil
}

func (s *server) RecordSubtopicInteraction(ctx context.Context, req *vectorv1.SubtopicInteraction) (*vectorv1.Ack, error) {
	_ = ctx
	if s.profiles == nil {
		return nil, status.Error(codes.FailedPrecondition, "profile store disabled")
	}
	if s.interactions == nil {
		return nil, status.Error(codes.FailedPrecondition, "subtopic interactions disabled")
	}
	if req.GetStudentId() == "" || req.GetParentNodeId() == "" || req.GetSubtopicId() == "" {
		return nil, status.Error(codes.InvalidArgument, "student_id, parent_node_id, and subtopic_id are required")
	}
	if s.reg == nil {
		return nil, status.Error(codes.FailedPrecondition, "interactive nodes disabled")
	}

	n, ok := s.reg.Get(req.GetParentNodeId())
	if !ok {
		return nil, status.Errorf(codes.NotFound, "interactive node %q not found", req.GetParentNodeId())
	}
	if n.Hierarchy == nil {
		return nil, status.Errorf(codes.NotFound, "node %q has no hierarchy", req.GetParentNodeId())
	}
	if _, found := n.Hierarchy.FindByID(req.GetSubtopicId()); !found {
		return nil, status.Errorf(codes.NotFound, "subtopic %q not found under %q", req.GetSubtopicId(), req.GetParentNodeId())
	}

	delta := dua.ResolveSubtopicDelta(n.Hierarchy, req.GetSubtopicId(), req.GetPreferenceDelta())
	s.interactions.Record(req.GetStudentId(), req.GetParentNodeId(), req.GetSubtopicId(), delta)
	return neutralAck(), nil
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

	var store *rag.Store
	var emb rag.Embedder
	if router.Enabled {
		store = rag.NewStore()
		emb = rag.DefaultEmbedder()
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

	profiles := dua.NewProfileStore()
	srvImpl := &server{
		router:       router,
		profiles:     profiles,
		interactions: dua.NewInteractionStoreWithProfiles(profiles),
	}

	if dua.EnabledFromEnv() {
		reg := dua.NewRegistry()
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
				fitted, err := vector.FitContentEmbedding(node.Embedding)
				if err != nil {
					log.Printf("index interactive node %s: %v", node.NodeID, err)
					return
				}
				if err := index.Upsert(vector.Node{
					ID:           node.NodeID,
					DimensionDUA: node.DimensionDUA,
					Difficulty:   "basico",
					Format:       "visual",
					ResourceURL:  "interactive://" + node.NodeID,
					Embedding:    fitted,
				}); err != nil {
					log.Printf("index interactive node %s: %v", node.NodeID, err)
				}
			})
		}
		srvImpl.reg = reg
		if store != nil && emb != nil {
			srvImpl.mutator = &dua.Mutator{
				Registry:  reg,
				Retriever: rag.NewRetriever(store, emb, 3),
			}
		} else {
			srvImpl.mutator = &dua.Mutator{Registry: reg}
		}
	} else {
		log.Printf("interactive nodes disabled (AVLP_INTERACTIVE_NODES=false)")
	}

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
}
