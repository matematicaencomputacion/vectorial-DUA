package routerserver

import (
	"context"
	"errors"
	"log"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	vectorv1 "github.com/vectorial-dua/avlp/gen/avlp/vector/v1"
	"github.com/vectorial-dua/avlp/pkg/dua"
	"github.com/vectorial-dua/avlp/pkg/knowledge"
	"github.com/vectorial-dua/avlp/pkg/rag"
	"github.com/vectorial-dua/avlp/pkg/rogerian"
	"github.com/vectorial-dua/avlp/pkg/session"
	"github.com/vectorial-dua/avlp/pkg/vector"
)

// Deps contains the domain services used by the gRPC transport adapter.
type Deps struct {
	Router        *vector.Router
	Registry      *dua.Registry
	Mutator       *dua.Mutator
	QueryEmbedder rag.Embedder
	Profiles      dua.ProfileRepository
	Interactions  *dua.InteractionStore
	Promoter      *dua.LiveStationPromoter
	Graph         knowledge.KnowledgeGraph
	Visits        knowledge.ConceptVisitStore
	Advisor       *knowledge.Advisor
}

// Server implements the VectorRouter gRPC service using the canonical domain services.
type Server struct {
	vectorv1.UnimplementedVectorRouterServer
	router        *vector.Router
	reg           *dua.Registry
	mutator       *dua.Mutator
	queryEmbedder rag.Embedder
	profiles      dua.ProfileRepository
	interactions  *dua.InteractionStore
	promoter      *dua.LiveStationPromoter
	graph         knowledge.KnowledgeGraph
	visits        knowledge.ConceptVisitStore
	advisor       *knowledge.Advisor
}

// New builds the canonical gRPC handler implementation.
func New(deps Deps) *Server {
	return &Server{
		router:        deps.Router,
		reg:           deps.Registry,
		mutator:       deps.Mutator,
		queryEmbedder: deps.QueryEmbedder,
		profiles:      deps.Profiles,
		interactions:  deps.Interactions,
		promoter:      deps.Promoter,
		graph:         deps.Graph,
		visits:        deps.Visits,
		advisor:       deps.Advisor,
	}
}

const ackRecorded = "recorded"
const promoteDeniedMessage = "No tenés permiso para esta acción."

func neutralAck() *vectorv1.Ack {
	return &vectorv1.Ack{Ok: true, Message: ackRecorded}
}

func (s *Server) QueryNearestNode(ctx context.Context, req *vectorv1.VectorQuery) (*vectorv1.RouteResult, error) {
	id, err := session.RequireSecureIdentity(ctx)
	if err != nil {
		return nil, err
	}
	studentID := ""
	var dims []float32
	preferredDimension := ""
	format := ""
	frustration := float32(0)
	frustrationProvided := false

	if st := req.GetStudentState(); st != nil {
		studentID = st.GetStudentId()
		dims = append([]float32(nil), st.GetDimensions()...)
		if sess := st.GetSession(); sess != nil {
			format = sess.GetPreferredFormat()
			preferredDimension = sess.GetPreferredDimensionDua()
			if sess.FrustrationSignal != nil {
				frustration = sess.GetFrustrationSignal()
				frustrationProvided = true
			}
		}
	}
	studentID, err = session.ResolveStudentID(id, studentID)
	if err != nil {
		return nil, err
	}
	if st := req.GetStudentState(); st != nil {
		st.StudentId = studentID
	}

	// Ola 1 merge rule (no blend): request dimensions win; otherwise profile store.
	if len(dims) == 0 && s.profiles != nil && studentID != "" {
		dims = s.profiles.Get(studentID)
	}

	dimension, resolvedFormat, resolvedFrustration := dua.ResolveRoutingHints(dua.ResolveRoutingHintsInput{
		Dimensions:          dims,
		PreferredDimension:  preferredDimension,
		PreferredFormat:     format,
		FrustrationSignal:   frustration,
		FrustrationProvided: frustrationProvided,
	})

	queryEmbedding := append([]float32(nil), req.GetQueryEmbedding()...)
	if len(queryEmbedding) == 0 && req.GetQueryText() != "" && s.queryEmbedder != nil {
		embedded, err := s.queryEmbedder.Embed(ctx, req.GetQueryText())
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "embed query_text: %v", err)
		}
		queryEmbedding = embedded
	}

	outcome, err := s.router.QueryNearestWithOptions(ctx, studentID, queryEmbedding, req.GetMinSimilarityThreshold(), vector.QueryOptions{
		DoubtText:   req.GetQueryText(),
		Frustration: resolvedFrustration,
		Dimension:   dimension,
		Format:      resolvedFormat,
	})
	if err != nil {
		return nil, err
	}

	if outcome.Matched {
		hasInteractive := false
		if s.reg != nil {
			_, hasInteractive = s.reg.Get(outcome.Node.ID)
		}
		conceptRefs := s.conceptRefsForMatch(outcome.Node)
		adviceES := s.adviceForMatch(ctx, studentID, conceptRefs)
		if err := knowledge.RecordConcepts(ctx, s.visits, studentID, conceptRefs); err != nil {
			log.Printf("concept visit record skipped student=%s node=%s: %v", studentID, outcome.Node.ID, err)
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
					AdviceEs:              adviceES,
				},
			},
		}, nil
	}

	return &vectorv1.RouteResult{
		Outcome: &vectorv1.RouteResult_Pending{
			Pending: &vectorv1.LiveStationPending{
				TrackingUlid: outcome.TrackingULID,
				Status:       outcome.LiveStatus,
				Message:      rogerian.LiveStationStudentMessage(outcome.LiveStatus, resolvedFrustration),
			},
		},
	}, nil
}

func (s *Server) GetLiveStation(ctx context.Context, req *vectorv1.LiveStationQuery) (*vectorv1.LiveStationStatus, error) {
	id, err := session.RequireSecureIdentity(ctx)
	if err != nil {
		return nil, err
	}
	if s.router == nil || s.router.Ledger == nil {
		return nil, status.Error(codes.FailedPrecondition, "station ledger unavailable")
	}
	if req.GetTrackingUlid() == "" {
		return nil, status.Error(codes.InvalidArgument, "tracking_ulid is required")
	}
	studentID, err := session.ResolveStudentID(id, req.GetStudentId())
	if err != nil {
		return nil, err
	}

	rec, err := s.router.LookupStation(ctx, req.GetTrackingUlid(), studentID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "lookup station: %v", err)
	}
	// Missing ULID and wrong student_id both map to NotFound (no existence leak).
	if rec == nil {
		return nil, status.Error(codes.NotFound, rogerian.LiveStationStudentMessage("expired", 0.5))
	}

	out := &vectorv1.LiveStationStatus{
		TrackingUlid:   rec.TrackingULID,
		Status:         rec.Status,
		StudentMessage: rogerian.LiveStationStudentMessage(rec.Status, rec.Request.Frustration),
	}
	if rec.Status == vector.StationReady && rec.Result != nil {
		out.NodeId = rec.Result.Node.ID
		out.LiveContent = rec.Result.Content
		out.RetrievedSources = append([]string(nil), rec.Result.Sources...)
	}
	return out, nil
}

func (s *Server) PromoteLiveStation(
	ctx context.Context,
	req *vectorv1.PromoteLiveStationRequest,
) (*vectorv1.PromoteLiveStationResponse, error) {
	id, err := session.RequireSecureIdentity(ctx)
	if err != nil {
		return nil, err
	}
	if id.Secure && id.Role != session.RoleTeacher {
		return nil, status.Error(codes.PermissionDenied, promoteDeniedMessage)
	}
	if req.GetTrackingUlid() == "" {
		return nil, status.Error(codes.InvalidArgument, "tracking_ulid is required")
	}
	if s.promoter == nil {
		return nil, status.Error(codes.FailedPrecondition, "live station promotion unavailable")
	}
	result, err := s.promoter.Promote(req.GetTrackingUlid())
	switch {
	case errors.Is(err, dua.ErrInvalidTrackingULID):
		return nil, status.Error(codes.InvalidArgument, "tracking_ulid must be a valid ULID")
	case errors.Is(err, dua.ErrStationNotFound):
		return nil, status.Error(codes.NotFound, "live station not found or expired")
	case errors.Is(err, dua.ErrStationNotReady):
		return nil, status.Error(codes.FailedPrecondition, "live station is not ready")
	case errors.Is(err, dua.ErrPromotionUnavailable):
		return nil, status.Error(codes.FailedPrecondition, "live station promotion unavailable")
	case err != nil:
		log.Printf("promote live station %s: %v", req.GetTrackingUlid(), err)
		return nil, status.Error(codes.Internal, "could not persist promoted station")
	}
	return &vectorv1.PromoteLiveStationResponse{
		TrackingUlid:     result.TrackingULID,
		NodeId:           result.Node.NodeID,
		SeedPath:         result.SeedPath,
		Created:          result.Created,
		LiveContent:      result.Node.StageMarkdownDefault,
		RetrievedSources: append([]string(nil), result.Node.RetrievedSources...),
		DimensionDua:     result.Node.DimensionDUA,
	}, nil
}

func (s *Server) GetInteractiveNode(ctx context.Context, req *vectorv1.NodeIdRequest) (*vectorv1.InteractiveVideoNode, error) {
	if _, err := session.RequireSecureIdentity(ctx); err != nil {
		return nil, err
	}
	if s.reg == nil {
		return nil, status.Error(codes.FailedPrecondition, "interactive nodes disabled")
	}
	n, ok := s.reg.Get(req.GetNodeId())
	if !ok {
		return nil, status.Errorf(codes.NotFound, "interactive node %q not found", req.GetNodeId())
	}
	return dua.ToProto(n), nil
}

func (s *Server) MutateInteractiveNode(ctx context.Context, req *vectorv1.MutateInteractiveRequest) (*vectorv1.MutateInteractiveResponse, error) {
	id, err := session.RequireSecureIdentity(ctx)
	if err != nil {
		return nil, err
	}
	studentID, err := session.ResolveStudentID(id, req.GetStudentId())
	if err != nil {
		return nil, err
	}
	if s.mutator == nil {
		return nil, status.Error(codes.FailedPrecondition, "interactive mutation disabled")
	}
	res, err := s.mutator.Mutate(ctx, dua.MutateRequest{
		NodeID:         req.GetNodeId(),
		StudentID:      studentID,
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

func (s *Server) RecordBotoneraInteraction(ctx context.Context, req *vectorv1.BotoneraInteraction) (*vectorv1.Ack, error) {
	id, err := session.RequireSecureIdentity(ctx)
	if err != nil {
		return nil, err
	}
	studentID, err := session.ResolveStudentID(id, req.GetStudentId())
	if err != nil {
		return nil, err
	}
	req.StudentId = studentID
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
	if !dua.HasBotoneraVariant(n, req.GetVariantId(), req.GetFormatId()) {
		return nil, status.Errorf(codes.NotFound, "variant %q not found in botonera for node %q", req.GetVariantId(), req.GetNodeId())
	}

	delta := dua.ResolveBotoneraDelta(n, req.GetVariantId(), req.GetPreferenceDelta())
	if len(delta) == 0 {
		_ = knowledge.RecordConcepts(ctx, s.visits, req.GetStudentId(), n.Concepts)
		return neutralAck(), nil
	}
	if _, err := s.profiles.Apply(req.GetStudentId(), delta); err != nil {
		log.Printf("botonera profile delta skipped student=%s node=%s variant=%s: %v",
			req.GetStudentId(), req.GetNodeId(), req.GetVariantId(), err)
	}
	if err := knowledge.RecordConcepts(ctx, s.visits, req.GetStudentId(), n.Concepts); err != nil {
		log.Printf("concept visit record skipped student=%s node=%s: %v", req.GetStudentId(), req.GetNodeId(), err)
	}
	return neutralAck(), nil
}

func (s *Server) RecordSubtopicInteraction(ctx context.Context, req *vectorv1.SubtopicInteraction) (*vectorv1.Ack, error) {
	id, err := session.RequireSecureIdentity(ctx)
	if err != nil {
		return nil, err
	}
	studentID, err := session.ResolveStudentID(id, req.GetStudentId())
	if err != nil {
		return nil, err
	}
	req.StudentId = studentID
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
	if err := knowledge.RecordConcepts(ctx, s.visits, req.GetStudentId(), n.Concepts); err != nil {
		log.Printf("concept visit record skipped student=%s node=%s: %v", req.GetStudentId(), req.GetParentNodeId(), err)
	}
	return neutralAck(), nil
}

func (s *Server) GetSubtopicProgress(ctx context.Context, req *vectorv1.SubtopicProgressQuery) (*vectorv1.SubtopicProgress, error) {
	id, err := session.RequireSecureIdentity(ctx)
	if err != nil {
		return nil, err
	}
	studentID, err := session.ResolveStudentID(id, req.GetStudentId())
	if err != nil {
		return nil, err
	}
	if req.GetParentNodeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "parent_node_id is required")
	}
	if s.interactions == nil {
		return nil, status.Error(codes.FailedPrecondition, "subtopic interactions disabled")
	}
	if s.reg == nil {
		return nil, status.Error(codes.FailedPrecondition, "interactive nodes disabled")
	}

	node, ok := s.reg.Get(req.GetParentNodeId())
	if !ok {
		return nil, status.Errorf(codes.NotFound, "interactive node %q not found", req.GetParentNodeId())
	}
	if node.Hierarchy == nil {
		return nil, status.Errorf(codes.NotFound, "node %q has no hierarchy", req.GetParentNodeId())
	}

	opened := s.interactions.OpenedList(studentID, req.GetParentNodeId())
	openedSet := make(map[string]struct{}, len(opened))
	for _, id := range opened {
		openedSet[id] = struct{}{}
	}
	progress := dua.ProgressForTree(node.Hierarchy, openedSet)

	out := &vectorv1.SubtopicProgress{
		StudentId:         studentID,
		ParentNodeId:      req.GetParentNodeId(),
		OpenedSubtopicIds: progress.OpenedSubtopicIDs,
		TotalSubtopics:    int32(progress.TotalSubtopics),
		RootStates:        make([]*vectorv1.RootSubtopicProgress, 0, len(progress.RootStates)),
		NodeStates:        make([]*vectorv1.NodeSubtopicProgress, 0, len(progress.NodeStates)),
	}
	for _, root := range progress.RootStates {
		out.RootStates = append(out.RootStates, &vectorv1.RootSubtopicProgress{
			SubtopicId: root.SubtopicID,
			Title:      root.Title,
			State:      string(root.State),
		})
	}
	for _, node := range progress.NodeStates {
		out.NodeStates = append(out.NodeStates, &vectorv1.NodeSubtopicProgress{
			SubtopicId:      node.SubtopicID,
			Title:           node.Title,
			State:           string(node.State),
			OpenedInSubtree: int32(node.OpenedInSubtree),
			TotalInSubtree:  int32(node.TotalInSubtree),
		})
	}
	return out, nil
}

func (s *Server) conceptRefsForMatch(node vector.Node) []string {
	if s.reg != nil {
		if n, ok := s.reg.Get(node.ID); ok && len(n.Concepts) > 0 {
			return append([]string(nil), n.Concepts...)
		}
	}
	return append([]string(nil), node.Concepts...)
}

func (s *Server) adviceForMatch(ctx context.Context, studentID string, refs []string) string {
	if s.advisor == nil || studentID == "" || len(refs) == 0 {
		return ""
	}
	var focuses []knowledge.ConceptID
	for _, raw := range refs {
		id, err := knowledge.NormalizeConceptRef(raw)
		if err != nil {
			continue
		}
		focuses = append(focuses, id)
	}
	if len(focuses) == 0 {
		return ""
	}
	adv, err := s.advisor.AdviseForConcepts(ctx, studentID, focuses)
	if err != nil {
		log.Printf("concept advice skipped student=%s: %v", studentID, err)
		return ""
	}
	return adv.MessageES
}
