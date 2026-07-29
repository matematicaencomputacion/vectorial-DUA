package webgateway_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	vectorv1 "github.com/vectorial-dua/avlp/gen/avlp/vector/v1"
	"github.com/vectorial-dua/avlp/internal/routerserver"
	"github.com/vectorial-dua/avlp/pkg/dua"
	"github.com/vectorial-dua/avlp/pkg/vector"
	"github.com/vectorial-dua/avlp/pkg/webgateway"
)

const bufSize = 1 << 20

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
			ID:           "dua::Representacion::adaptativo::conceptual::01TESTGATEWAYLIVE0000",
			DimensionDUA: "Representacion",
			Format:       "conceptual",
			Embedding:    emb,
		},
		Content:      "# Estación\n\ncontenido live",
		Sources:      []string{"kb/a.md"},
		TrackingULID: req.TrackingULID,
	}, nil
}

type staticQueryEmbedder struct{}

func (staticQueryEmbedder) Embed(context.Context, string) ([]float32, error) {
	return novelEmb(), nil
}

func (staticQueryEmbedder) Dims() int { return vector.ContentEmbedDims }

type testBackend struct {
	live     *testLive
	profiles dua.ProfileRepository
}

func novelEmb() []float32 {
	q := make([]float32, vector.ContentEmbedDims)
	q[len(q)-1] = 1
	return q
}

func seedDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "data", "nodes", "interactive"))
}

func startTestGateway(t *testing.T) (*webgateway.Gateway, *testBackend, func()) {
	t.Helper()
	idx := vector.NewIndex()
	r := vector.NewRouter(idx, vector.NewEventBus())
	live := &testLive{fail: true}
	r.Enabled = true
	r.Live = live

	reg := dua.NewRegistry()
	if _, err := reg.LoadDir(seedDir(t)); err != nil {
		t.Fatal(err)
	}
	profiles := dua.NewProfileStore()
	interactions := dua.NewInteractionStoreWithProfiles(profiles)
	impl := routerserver.New(routerserver.Deps{
		Router:        r,
		Registry:      reg,
		Mutator:       &dua.Mutator{Registry: reg},
		QueryEmbedder: staticQueryEmbedder{},
		Profiles:      profiles,
		Interactions:  interactions,
	})
	backend := &testBackend{live: live, profiles: profiles}

	lis := bufconn.Listen(bufSize)
	gs := grpc.NewServer()
	vectorv1.RegisterVectorRouterServer(gs, impl)
	go func() { _ = gs.Serve(lis) }()

	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	client := vectorv1.NewVectorRouterClient(conn)
	gw := webgateway.New(client, nil)
	cleanup := func() {
		_ = conn.Close()
		gs.Stop()
		_ = lis.Close()
	}
	return gw, backend, cleanup
}

func TestGatewayQueryPendingThenPollReady(t *testing.T) {
	gw, impl, cleanup := startTestGateway(t)
	defer cleanup()

	body := `{"student_id":"stu-web","query_text":"duda sin nodo cercano"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/query", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	gw.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("query status=%d body=%s", rr.Code, rr.Body.String())
	}
	var route map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &route); err != nil {
		t.Fatal(err)
	}
	pending, ok := route["pending"].(map[string]any)
	if !ok {
		t.Fatalf("expected pending, got %s", rr.Body.String())
	}
	ulid, _ := pending["tracking_ulid"].(string)
	msg, _ := pending["message"].(string)
	if ulid == "" || msg == "" {
		t.Fatalf("pending incomplete: %v", pending)
	}
	if strings.Contains(msg, "threshold") {
		t.Fatalf("technical English leaked: %q", msg)
	}

	// Still failing → failed or in_progress with rogerian message
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/stations/"+ulid+"?student_id=stu-web", nil)
	gw.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("poll status=%d body=%s", rr.Code, rr.Body.String())
	}

	impl.live.fail = false
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/stations/"+ulid+"?student_id=stu-web", nil)
	gw.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("poll ready status=%d body=%s", rr.Code, rr.Body.String())
	}
	var st map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	if st["status"] != "ready" {
		t.Fatalf("want ready, got %v", st["status"])
	}
	if st["live_content"] == "" {
		t.Fatal("expected live_content")
	}
}

func TestGatewayGetNodeAndMutate(t *testing.T) {
	gw, _, cleanup := startTestGateway(t)
	defer cleanup()

	nodeID := "dua::Representacion::basico::visual::01ARZ3NDEKTSV4RRFFQ69G5FAV"
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/nodes/"+nodeID, nil)
	gw.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("get node status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "botonera_schema") && !strings.Contains(rr.Body.String(), `"titulo"`) {
		t.Fatalf("expected interactive node JSON, got %s", rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/nodes/"+nodeID+"/mutate",
		strings.NewReader(`{"student_id":"stu-web","doubt_text":"otra duda sobre scope"}`))
	req.Header.Set("Content-Type", "application/json")
	gw.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("mutate status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "live_") && !strings.Contains(rr.Body.String(), "LIVE:") {
		t.Fatalf("expected live button in mutate response: %s", rr.Body.String())
	}
}

func TestGatewaySubtopicProgress(t *testing.T) {
	gw, _, cleanup := startTestGateway(t)
	defer cleanup()

	const nodeID = "dua::Representacion::basico::visual::01ARZ3NDEKTSV4RRFFQ69G5FC0"
	const studentID = "stu-progress-web"
	progressURL := "/api/nodes/" + nodeID + "/progress?student_id=" + studentID

	getProgress := func() *vectorv1.SubtopicProgress {
		t.Helper()
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, progressURL, nil)
		gw.Handler().ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("progress status=%d body=%s", rr.Code, rr.Body.String())
		}
		var got vectorv1.SubtopicProgress
		if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		return &got
	}

	empty := getProgress()
	if empty.GetTotalSubtopics() != 5 || len(empty.GetOpenedSubtopicIds()) != 0 {
		t.Fatalf("empty progress=%+v", empty)
	}

	for _, subtopicID := range []string{"sub_motor", "sub_4_ruedas"} {
		body := `{"student_id":"` + studentID + `","parent_node_id":"` + nodeID + `","subtopic_id":"` + subtopicID + `"}`
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/interactions/subtopic", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		gw.Handler().ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("record %s status=%d body=%s", subtopicID, rr.Code, rr.Body.String())
		}
	}

	got := getProgress()
	if opened := got.GetOpenedSubtopicIds(); len(opened) != 2 || opened[0] != "sub_motor" || opened[1] != "sub_4_ruedas" {
		t.Fatalf("opened=%v", got.GetOpenedSubtopicIds())
	}
	roots := got.GetRootStates()
	if len(roots) != 2 || roots[0].GetState() != "partial" || roots[1].GetState() != "visited" {
		t.Fatalf("roots=%+v", roots)
	}

	missingStudent := httptest.NewRecorder()
	gw.Handler().ServeHTTP(missingStudent, httptest.NewRequest(http.MethodGet, "/api/nodes/"+nodeID+"/progress", nil))
	if missingStudent.Code != http.StatusBadRequest {
		t.Fatalf("missing student status=%d body=%s", missingStudent.Code, missingStudent.Body.String())
	}

	noHierarchy := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/api/nodes/dua::Representacion::basico::visual::01ARZ3NDEKTSV4RRFFQ69G5FAV/progress?student_id="+studentID, nil)
	gw.Handler().ServeHTTP(noHierarchy, req)
	if noHierarchy.Code != http.StatusNotFound {
		t.Fatalf("no hierarchy status=%d body=%s", noHierarchy.Code, noHierarchy.Body.String())
	}
}

func TestGatewayMappedErrors(t *testing.T) {
	gw, _, cleanup := startTestGateway(t)
	defer cleanup()

	// Missing query fields → 400 (gateway)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/query", strings.NewReader(`{"student_id":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	gw.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rr.Code)
	}

	// Unknown node → 404 with student_message
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/nodes/does-not-exist", nil)
	gw.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d body=%s", rr.Code, rr.Body.String())
	}
	var errBody map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &errBody); err != nil {
		t.Fatal(err)
	}
	if errBody["student_message"] == "" {
		t.Fatalf("expected student_message in 404 body: %v", errBody)
	}

	// Station wrong student → 404
	body := `{"student_id":"owner","query_text":"miss"}`
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/query", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	gw.Handler().ServeHTTP(rr, req)
	var route map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &route)
	ulid, _ := route["pending"].(map[string]any)["tracking_ulid"].(string)
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/stations/"+ulid+"?student_id=intruder", nil)
	gw.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("wrong student want 404, got %d %s", rr.Code, rr.Body.String())
	}

	// Botonera missing variant_id → 400
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/interactions/botonera",
		bytes.NewReader([]byte(`{"student_id":"s","node_id":"n"}`)))
	req.Header.Set("Content-Type", "application/json")
	gw.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("botonera want 400, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestGatewayStaticPlaceholder(t *testing.T) {
	static := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, "<!doctype html><title>AVLP</title>")
	})
	gw := webgateway.New(nil, static)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	gw.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "AVLP") {
		t.Fatalf("static: %d %s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("index Cache-Control: want no-cache, got %q", got)
	}

	rr2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/index.html", nil)
	gw.Handler().ServeHTTP(rr2, req2)
	if got := rr2.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("/index.html Cache-Control: want no-cache, got %q", got)
	}
}

func TestGatewayBotoneraRecord(t *testing.T) {
	gw, impl, cleanup := startTestGateway(t)
	defer cleanup()

	nodeID := "dua::Representacion::basico::visual::01ARZ3NDEKTSV4RRFFQ69G5FAV"
	payload := `{"node_id":"` + nodeID + `","student_id":"stu-ve","schema_kind":"depth","variant_id":"core","preference_delta":[0.1,0,0,0,0]}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/interactions/botonera", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	gw.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("botonera status=%d body=%s", rr.Code, rr.Body.String())
	}
	ve := impl.profiles.Get("stu-ve")
	if ve[0] < 0.1 {
		t.Fatalf("expected V_e updated, got %v", ve)
	}
}
