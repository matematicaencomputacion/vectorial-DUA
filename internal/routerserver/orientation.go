package routerserver

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	vectorv1 "github.com/vectorial-dua/avlp/gen/avlp/vector/v1"
	"github.com/vectorial-dua/avlp/pkg/knowledge"
	"github.com/vectorial-dua/avlp/pkg/session"
)

func (s *Server) GetNodeOrientation(ctx context.Context, req *vectorv1.NodeOrientationQuery) (*vectorv1.NodeOrientation, error) {
	id, err := session.RequireSecureIdentity(ctx)
	if err != nil {
		return nil, err
	}
	studentID, err := session.ResolveStudentID(id, req.GetStudentId())
	if err != nil {
		return nil, err
	}
	if req.GetNodeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "node_id is required")
	}

	out := &vectorv1.NodeOrientation{
		StudentId: studentID,
		NodeId:    req.GetNodeId(),
		Available: false,
	}
	if s.advisor == nil {
		return out, nil
	}
	if s.reg != nil {
		if node, ok := s.reg.Get(req.GetNodeId()); ok {
			return s.orientationFromRefs(ctx, studentID, req.GetNodeId(), node.Concepts)
		}
	}
	if refs := s.indexConcepts(req.GetNodeId()); refs != nil {
		return s.orientationFromRefs(ctx, studentID, req.GetNodeId(), refs)
	}
	return nil, status.Errorf(codes.NotFound, "node %q not found", req.GetNodeId())
}

func (s *Server) orientationFromRefs(
	ctx context.Context,
	studentID, nodeID string,
	refs []string,
) (*vectorv1.NodeOrientation, error) {
	out := &vectorv1.NodeOrientation{
		StudentId: studentID,
		NodeId:    nodeID,
		Available: false,
	}
	if s.advisor == nil {
		return out, nil
	}
	var focuses []knowledge.ConceptID
	for _, raw := range refs {
		id, err := knowledge.NormalizeConceptRef(raw)
		if err != nil {
			continue
		}
		focuses = append(focuses, id)
		out.FocusConceptIds = append(out.FocusConceptIds, string(id))
	}
	if len(focuses) == 0 {
		// Node has no concepts: orientation is available and empty.
		out.Available = true
		return out, nil
	}
	adv, err := s.advisor.AdviseForConcepts(ctx, studentID, focuses)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	out.Available = adv.Available
	out.MessageEs = adv.MessageES
	for _, g := range adv.Gaps {
		out.Gaps = append(out.Gaps, &vectorv1.ConceptGap{
			ConceptId:   string(g.Peer.ID),
			Title:       g.Peer.Title,
			Track:       string(g.Peer.Track),
			Kind:        string(g.Kind),
			Strength:    g.Strength,
			RationaleEs: g.RationaleES,
			Depth:       int32(g.Depth),
		})
	}
	return out, nil
}

func (s *Server) indexConcepts(nodeID string) []string {
	if s.router == nil || s.router.Index == nil {
		return nil
	}
	n, ok := s.router.Index.Get(nodeID)
	if !ok {
		return nil
	}
	return append([]string(nil), n.Concepts...)
}

func (s *Server) GetConceptRoute(ctx context.Context, req *vectorv1.ConceptRouteQuery) (*vectorv1.ConceptRoute, error) {
	id, err := session.RequireSecureIdentity(ctx)
	if err != nil {
		return nil, err
	}
	studentID, err := session.ResolveStudentID(id, req.GetStudentId())
	if err != nil {
		return nil, err
	}
	out := &vectorv1.ConceptRoute{
		StudentId: studentID,
		Available: false,
	}
	if s.advisor == nil {
		return out, nil
	}
	from, err := knowledge.NormalizeConceptRef(req.GetFromConceptId())
	if err != nil || req.GetFromConceptId() == "" {
		return nil, status.Error(codes.InvalidArgument, "from_concept_id is required")
	}
	to, err := knowledge.NormalizeConceptRef(req.GetToConceptId())
	if err != nil || req.GetToConceptId() == "" {
		return nil, status.Error(codes.InvalidArgument, "to_concept_id is required")
	}
	route, err := s.advisor.Route(ctx, from, to)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	out.Available = route.Available
	for _, c := range route.Path {
		out.ConceptIds = append(out.ConceptIds, string(c.ID))
		out.Titles = append(out.Titles, c.Title)
	}
	return out, nil
}
