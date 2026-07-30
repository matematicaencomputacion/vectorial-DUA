package main

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	vectorv1 "github.com/vectorial-dua/avlp/gen/avlp/vector/v1"
	"github.com/vectorial-dua/avlp/internal/routerserver"
	"github.com/vectorial-dua/avlp/pkg/dua"
	"github.com/vectorial-dua/avlp/pkg/vector"
)

func TestPromoteLiveStationSuccessAndReplay(t *testing.T) {
	idx := vector.NewIndex()
	ledger := vector.NewStationLedger(0)
	reg := dua.NewRegistry()
	tracking, node := readyPromotionStation(t, idx, ledger)
	srv := routerserver.New(routerserver.Deps{
		Promoter: &dua.LiveStationPromoter{
			Ledger:   ledger,
			Index:    idx,
			Registry: reg,
			SeedsDir: t.TempDir(),
		},
	})

	first, err := srv.PromoteLiveStation(context.Background(), &vectorv1.PromoteLiveStationRequest{
		TrackingUlid: tracking,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !first.GetCreated() || first.GetNodeId() != node.ID ||
		first.GetLiveContent() == "" || first.GetSeedPath() == "" {
		t.Fatalf("first=%+v", first)
	}
	replay, err := srv.PromoteLiveStation(context.Background(), &vectorv1.PromoteLiveStationRequest{
		TrackingUlid: tracking,
	})
	if err != nil {
		t.Fatal(err)
	}
	if replay.GetCreated() || replay.GetNodeId() != first.GetNodeId() {
		t.Fatalf("replay=%+v", replay)
	}
}

func TestPromoteLiveStationValidationAndStates(t *testing.T) {
	idx := vector.NewIndex()
	ledger := vector.NewStationLedger(0)
	srv := routerserver.New(routerserver.Deps{
		Promoter: &dua.LiveStationPromoter{
			Ledger:   ledger,
			Index:    idx,
			Registry: dua.NewRegistry(),
			SeedsDir: t.TempDir(),
		},
	})

	tests := []struct {
		name string
		ulid string
		code codes.Code
	}{
		{name: "required", ulid: "", code: codes.InvalidArgument},
		{name: "invalid", ulid: "../escape", code: codes.InvalidArgument},
	}
	missing, err := vector.NewTrackingULID()
	if err != nil {
		t.Fatal(err)
	}
	tests = append(tests, struct {
		name string
		ulid string
		code codes.Code
	}{name: "missing", ulid: missing, code: codes.NotFound})
	pending, err := vector.NewTrackingULID()
	if err != nil {
		t.Fatal(err)
	}
	ledger.RegisterInProgress(pending, "stu", vector.LiveRequest{DoubtText: "pendiente"})
	tests = append(tests, struct {
		name string
		ulid string
		code codes.Code
	}{name: "pending", ulid: pending, code: codes.FailedPrecondition})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := srv.PromoteLiveStation(context.Background(), &vectorv1.PromoteLiveStationRequest{
				TrackingUlid: tt.ulid,
			})
			if status.Code(err) != tt.code {
				t.Fatalf("code=%s want=%s err=%v", status.Code(err), tt.code, err)
			}
		})
	}
}

func readyPromotionStation(
	t *testing.T,
	idx *vector.Index,
	ledger *vector.StationLedger,
) (string, vector.Node) {
	t.Helper()
	tracking, err := vector.NewTrackingULID()
	if err != nil {
		t.Fatal(err)
	}
	embedding := make([]float32, idx.Dims())
	embedding[0] = 1
	node, err := idx.RegisterLiveNode(
		"Representacion",
		"adaptativo",
		"visual",
		"live://stations/"+tracking,
		embedding,
	)
	if err != nil {
		t.Fatal(err)
	}
	ledger.RegisterInProgress(tracking, "stu", vector.LiveRequest{
		DoubtText:      "Explicación que vale la pena conservar",
		QueryEmbedding: embedding,
		Dimension:      "Representacion",
		Format:         "visual",
	})
	ledger.MarkReady(tracking, vector.LiveResult{
		Node:         node,
		Content:      "# Estación promovida",
		Sources:      []string{"kb/source.md"},
		TrackingULID: tracking,
	})
	return tracking, node
}
