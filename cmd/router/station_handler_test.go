package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	vectorv1 "github.com/vectorial-dua/avlp/gen/avlp/vector/v1"
	"github.com/vectorial-dua/avlp/internal/routerserver"
	"github.com/vectorial-dua/avlp/pkg/vector"
)

type testLive struct {
	fail bool
}

func (t *testLive) GenerateLive(ctx context.Context, req vector.LiveRequest) (vector.LiveResult, error) {
	_ = ctx
	if t.fail {
		return vector.LiveResult{}, errors.New("offline")
	}
	emb := make([]float32, vector.ContentEmbedDims)
	emb[0] = 1
	return vector.LiveResult{
		Node: vector.Node{
			ID:           "dua::Representacion::adaptativo::conceptual::01TESTGETLIVESTATION00",
			DimensionDUA: "Representacion",
			Format:       "conceptual",
			Embedding:    emb,
		},
		Content:      "# Estación\n\ncontenido",
		Sources:      []string{"kb/a.md"},
		TrackingULID: req.TrackingULID,
	}, nil
}

func novelEmb() []float32 {
	q := make([]float32, vector.ContentEmbedDims)
	q[len(q)-1] = 1
	return q
}

func TestGetLiveStationPendingThenReady(t *testing.T) {
	idx := vector.NewIndex()
	r := vector.NewRouter(idx, vector.NewEventBus())
	r.Enabled = true
	live := &testLive{fail: true}
	r.Live = live
	srv := routerserver.New(routerserver.Deps{Router: r})

	route, err := srv.QueryNearestNode(context.Background(), &vectorv1.VectorQuery{
		StudentState:           &vectorv1.StudentVector{StudentId: "stu-a"},
		QueryEmbedding:         novelEmb(),
		MinSimilarityThreshold: 0.85,
		QueryText:              "duda nueva",
	})
	if err != nil {
		t.Fatal(err)
	}
	pending := route.GetPending()
	if pending == nil || pending.GetTrackingUlid() == "" {
		t.Fatalf("expected pending, got %+v", route)
	}
	techEN := "No static DUA node met the similarity threshold; live station generation requested"
	if pending.GetMessage() == techEN || pending.GetMessage() == "" {
		t.Fatalf("expected rogerian Spanish message, got %q", pending.GetMessage())
	}
	if !strings.Contains(pending.GetMessage(), "estación") && !strings.Contains(pending.GetMessage(), "material") {
		t.Fatalf("expected Spanish student copy, got %q", pending.GetMessage())
	}

	st, err := srv.GetLiveStation(context.Background(), &vectorv1.LiveStationQuery{
		TrackingUlid: pending.GetTrackingUlid(),
		StudentId:    "stu-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	if st.GetStatus() != vector.StationFailed && st.GetStatus() != vector.StationInProgress {
		t.Fatalf("status=%s", st.GetStatus())
	}
	if st.GetStudentMessage() == "" {
		t.Fatal("expected student_message")
	}

	live.fail = false
	st, err = srv.GetLiveStation(context.Background(), &vectorv1.LiveStationQuery{
		TrackingUlid: pending.GetTrackingUlid(),
		StudentId:    "stu-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	if st.GetStatus() != vector.StationReady || st.GetLiveContent() == "" || st.GetNodeId() == "" {
		t.Fatalf("expected ready payload, got %+v", st)
	}
}

func TestGetLiveStationNotFoundMissingAndWrongStudent(t *testing.T) {
	idx := vector.NewIndex()
	r := vector.NewRouter(idx, vector.NewEventBus())
	r.Enabled = false
	srv := routerserver.New(routerserver.Deps{Router: r})

	route, err := srv.QueryNearestNode(context.Background(), &vectorv1.VectorQuery{
		StudentState:           &vectorv1.StudentVector{StudentId: "owner"},
		QueryEmbedding:         novelEmb(),
		MinSimilarityThreshold: 0.85,
	})
	if err != nil {
		t.Fatal(err)
	}
	ulid := route.GetPending().GetTrackingUlid()

	_, err = srv.GetLiveStation(context.Background(), &vectorv1.LiveStationQuery{
		TrackingUlid: "01NOTAREALULID000000000000",
		StudentId:    "owner",
	})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("missing ulid: want NotFound, got %v", err)
	}

	_, err = srv.GetLiveStation(context.Background(), &vectorv1.LiveStationQuery{
		TrackingUlid: ulid,
		StudentId:    "intruder",
	})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("wrong student: want NotFound, got %v", err)
	}
}
