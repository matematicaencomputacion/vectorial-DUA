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
)

const progressBufSize = 1024 * 1024

const automovilNodeID = "dua::Representacion::basico::visual::01ARZ3NDEKTSV4RRFFQ69G5FC0"

func routerSeedDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "data", "nodes", "interactive"))
}

func startProgressClient(t *testing.T) vectorv1.VectorRouterClient {
	t.Helper()
	reg := dua.NewRegistry()
	if _, err := reg.LoadDir(routerSeedDir(t)); err != nil {
		t.Fatal(err)
	}
	profiles := dua.NewProfileStore()
	impl := routerserver.New(routerserver.Deps{
		Registry:     reg,
		Profiles:     profiles,
		Interactions: dua.NewInteractionStoreWithProfiles(profiles),
	})

	lis := bufconn.Listen(progressBufSize)
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

func TestGetSubtopicProgressBufconnEmptyAndAfterRecords(t *testing.T) {
	client := startProgressClient(t)
	ctx := context.Background()
	query := &vectorv1.SubtopicProgressQuery{
		StudentId:    "stu-progress",
		ParentNodeId: automovilNodeID,
	}

	empty, err := client.GetSubtopicProgress(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	if empty.GetTotalSubtopics() != 5 || len(empty.GetOpenedSubtopicIds()) != 0 {
		t.Fatalf("empty progress=%+v", empty)
	}
	if len(empty.GetRootStates()) != 2 ||
		empty.GetRootStates()[0].GetState() != string(dua.ProgressUnvisited) ||
		empty.GetRootStates()[1].GetState() != string(dua.ProgressUnvisited) {
		t.Fatalf("empty root states=%+v", empty.GetRootStates())
	}

	for _, id := range []string{"sub_motor", "sub_4_ruedas"} {
		_, err := client.RecordSubtopicInteraction(ctx, &vectorv1.SubtopicInteraction{
			StudentId:    query.StudentId,
			ParentNodeId: query.ParentNodeId,
			SubtopicId:   id,
		})
		if err != nil {
			t.Fatalf("record %s: %v", id, err)
		}
	}

	got, err := client.GetSubtopicProgress(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	if got.GetTotalSubtopics() != 5 {
		t.Fatalf("TotalSubtopics=%d want=5", got.GetTotalSubtopics())
	}
	opened := got.GetOpenedSubtopicIds()
	if len(opened) != 2 || opened[0] != "sub_motor" || opened[1] != "sub_4_ruedas" {
		t.Fatalf("OpenedSubtopicIds=%v", opened)
	}
	roots := got.GetRootStates()
	if len(roots) != 2 ||
		roots[0].GetSubtopicId() != "sub_caja_central" || roots[0].GetState() != string(dua.ProgressPartial) ||
		roots[1].GetSubtopicId() != "sub_4_ruedas" || roots[1].GetState() != string(dua.ProgressVisited) {
		t.Fatalf("RootStates=%+v", roots)
	}
}

func TestGetSubtopicProgressBufconnValidationAndNotFound(t *testing.T) {
	client := startProgressClient(t)
	ctx := context.Background()

	tests := []struct {
		name string
		req  *vectorv1.SubtopicProgressQuery
		code codes.Code
	}{
		{
			name: "required fields",
			req:  &vectorv1.SubtopicProgressQuery{},
			code: codes.InvalidArgument,
		},
		{
			name: "missing node",
			req: &vectorv1.SubtopicProgressQuery{
				StudentId:    "stu-progress",
				ParentNodeId: "dua::Representacion::basico::visual::01MISSINGNODE0000000000000",
			},
			code: codes.NotFound,
		},
		{
			name: "node without hierarchy",
			req: &vectorv1.SubtopicProgressQuery{
				StudentId:    "stu-progress",
				ParentNodeId: "dua::Representacion::basico::visual::01ARZ3NDEKTSV4RRFFQ69G5FAV",
			},
			code: codes.NotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.GetSubtopicProgress(ctx, tt.req)
			if status.Code(err) != tt.code {
				t.Fatalf("code=%s want=%s err=%v", status.Code(err), tt.code, err)
			}
		})
	}
}
