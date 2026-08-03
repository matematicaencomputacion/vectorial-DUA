package main

import (
	"context"
	"net"
	"path/filepath"
	"runtime"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	vectorv1 "github.com/vectorial-dua/avlp/gen/avlp/vector/v1"
	"github.com/vectorial-dua/avlp/internal/routerserver"
	"github.com/vectorial-dua/avlp/pkg/dua"
	"github.com/vectorial-dua/avlp/pkg/knowledge"
)

const orientationBufSize = 1024 * 1024

func curriculumPath(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "data", "knowledge", "curriculum.json"))
}

func startOrientationClient(t *testing.T, withAdvisor bool) vectorv1.VectorRouterClient {
	t.Helper()
	reg := dua.NewRegistry()
	if _, err := reg.LoadDir(routerSeedDir(t)); err != nil {
		t.Fatal(err)
	}
	profiles := dua.NewProfileStore()
	deps := routerserver.Deps{
		Registry:     reg,
		Profiles:     profiles,
		Interactions: dua.NewInteractionStoreWithProfiles(profiles),
		Visits:       knowledge.NewMemoryConceptVisitStore(),
	}
	if withAdvisor {
		g, _, err := knowledge.LoadFile(curriculumPath(t), knowledge.LoadOptions{})
		if err != nil {
			t.Fatal(err)
		}
		deps.Graph = g
		deps.Advisor = &knowledge.Advisor{Graph: g, Visits: deps.Visits}
	}
	impl := routerserver.New(deps)

	lis := bufconn.Listen(orientationBufSize)
	grpcServer := grpc.NewServer()
	vectorv1.RegisterVectorRouterServer(grpcServer, impl)
	go func() { _ = grpcServer.Serve(lis) }()

	conn, err := grpc.DialContext(context.Background(), "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
		grpcServer.Stop()
		_ = lis.Close()
	})
	return vectorv1.NewVectorRouterClient(conn)
}

func TestGetNodeOrientationNoAdvisorAvailableFalse(t *testing.T) {
	client := startOrientationClient(t, false)
	ctx := context.Background()
	res, err := client.GetNodeOrientation(ctx, &vectorv1.NodeOrientationQuery{
		StudentId: "stu",
		NodeId:    automovilNodeID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.GetAvailable() || res.GetMessageEs() != "" {
		t.Fatalf("want available=false empty, got %+v", res)
	}
}

func TestGetNodeOrientationWithGaps(t *testing.T) {
	client := startOrientationClient(t, true)
	ctx := context.Background()
	// debug-emergency teaches debug-variables which requires variables-scope.
	nodeID := "dua::Accion::basico::practica::01ARZ3NDEKTSV4RRFFQ69G5FB1"
	res, err := client.GetNodeOrientation(ctx, &vectorv1.NodeOrientationQuery{
		StudentId: "stu-new",
		NodeId:    nodeID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.GetAvailable() {
		t.Fatal("expected available")
	}
	if res.GetMessageEs() == "" || len(res.GetGaps()) == 0 {
		t.Fatalf("expected gaps, got %+v", res)
	}
}

func TestGetConceptRouteLearningOrder(t *testing.T) {
	client := startOrientationClient(t, true)
	ctx := context.Background()
	res, err := client.GetConceptRoute(ctx, &vectorv1.ConceptRouteQuery{
		StudentId:     "stu",
		FromConceptId: "concept:env-secrets",
		ToConceptId:   "concept:string-literals",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.GetAvailable() || len(res.GetConceptIds()) < 2 {
		t.Fatalf("route=%+v", res)
	}
	if res.GetConceptIds()[0] != "concept:string-literals" {
		t.Fatalf("learning order should start foundational: %v", res.GetConceptIds())
	}
	if res.GetConceptIds()[len(res.GetConceptIds())-1] != "concept:env-secrets" {
		t.Fatalf("learning order should end at focus: %v", res.GetConceptIds())
	}
}

func TestGetNodeOrientationValidation(t *testing.T) {
	client := startOrientationClient(t, true)
	_, err := client.GetNodeOrientation(context.Background(), &vectorv1.NodeOrientationQuery{
		StudentId: "stu",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code=%v err=%v", status.Code(err), err)
	}
}
