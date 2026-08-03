package webgateway_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
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
	"google.golang.org/protobuf/encoding/protojson"

	vectorv1 "github.com/vectorial-dua/avlp/gen/avlp/vector/v1"
	"github.com/vectorial-dua/avlp/internal/routerserver"
	"github.com/vectorial-dua/avlp/pkg/dua"
	"github.com/vectorial-dua/avlp/pkg/session"
	"github.com/vectorial-dua/avlp/pkg/stt"
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
			ID: vector.FormatNodeID(vector.NodeIDParts{
				Dimension:  "Representacion",
				Difficulty: "adaptativo",
				Format:     "conceptual",
				ULID:       req.TrackingULID,
			}),
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
	index    *vector.Index
	registry *dua.Registry
	seedsDir string
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
	seedsDir := t.TempDir()
	impl := routerserver.New(routerserver.Deps{
		Router:        r,
		Registry:      reg,
		Mutator:       &dua.Mutator{Registry: reg},
		QueryEmbedder: staticQueryEmbedder{},
		Profiles:      profiles,
		Interactions:  interactions,
		Promoter: &dua.LiveStationPromoter{
			Ledger:   r.Ledger,
			Index:    idx,
			Registry: reg,
			SeedsDir: seedsDir,
		},
	})
	backend := &testBackend{
		live:     live,
		profiles: profiles,
		index:    idx,
		registry: reg,
		seedsDir: seedsDir,
	}

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

func TestGatewayNodeOrientationAndConceptRoute(t *testing.T) {
	gw, _, cleanup := startTestGateway(t)
	defer cleanup()

	const nodeID = "dua::Accion::basico::practica::01ARZ3NDEKTSV4RRFFQ69G5FB1"
	const studentID = "stu-orient"

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/nodes/"+nodeID+"/orientation?student_id="+studentID, nil)
	gw.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("orientation status=%d body=%s", rr.Code, rr.Body.String())
	}
	var orient vectorv1.NodeOrientation
	if err := json.Unmarshal(rr.Body.Bytes(), &orient); err != nil {
		t.Fatal(err)
	}
	// Test gateway has no Advisor wired → available=false is intentional.
	if orient.GetAvailable() {
		t.Fatalf("expected available=false without advisor, got %+v", orient)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet,
		"/api/concepts/route?student_id="+studentID+"&from=concept:env-secrets&to=concept:string-literals", nil)
	gw.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("route status=%d body=%s", rr.Code, rr.Body.String())
	}
	var route vectorv1.ConceptRoute
	if err := json.Unmarshal(rr.Body.Bytes(), &route); err != nil {
		t.Fatal(err)
	}
	if route.GetAvailable() {
		t.Fatalf("expected available=false without advisor, got %+v", route)
	}

	missing := httptest.NewRecorder()
	gw.Handler().ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/api/concepts/route?student_id="+studentID, nil))
	if missing.Code != http.StatusBadRequest {
		t.Fatalf("missing from/to status=%d body=%s", missing.Code, missing.Body.String())
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
	if len(empty.GetNodeStates()) != 5 {
		t.Fatalf("empty node states=%+v", empty.GetNodeStates())
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
	nodes := got.GetNodeStates()
	if len(nodes) != 5 ||
		nodes[0].GetSubtopicId() != "sub_caja_central" ||
		nodes[0].GetState() != "partial" ||
		nodes[0].GetOpenedInSubtree() != 1 ||
		nodes[0].GetTotalInSubtree() != 4 ||
		nodes[3].GetSubtopicId() != "sub_motor" ||
		nodes[3].GetState() != "visited" {
		t.Fatalf("nodes=%+v", nodes)
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

func TestGatewayPromoteLiveStationRoundTrip(t *testing.T) {
	gw, backend, cleanup := startTestGateway(t)
	defer cleanup()
	backend.live.fail = false

	query := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/query",
		strings.NewReader(`{"student_id":"teacher-demo","query_text":"una duda novel para promover"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	gw.Handler().ServeHTTP(query, req)
	if query.Code != http.StatusOK {
		t.Fatalf("query status=%d body=%s", query.Code, query.Body.String())
	}
	var route vectorv1.RouteResult
	if err := protojson.Unmarshal(query.Body.Bytes(), &route); err != nil {
		t.Fatal(err)
	}
	matched := route.GetMatched()
	if matched == nil || matched.GetNodeId() == "" {
		t.Fatalf("route=%+v", &route)
	}
	parts, err := vector.ParseNodeID(matched.GetNodeId())
	if err != nil {
		t.Fatal(err)
	}

	promoteURL := "/api/stations/" + parts.ULID + "/promote"
	promote := func() *vectorv1.PromoteLiveStationResponse {
		t.Helper()
		rr := httptest.NewRecorder()
		gw.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, promoteURL, nil))
		if rr.Code != http.StatusOK {
			t.Fatalf("promote status=%d body=%s", rr.Code, rr.Body.String())
		}
		var got vectorv1.PromoteLiveStationResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		return &got
	}

	first := promote()
	if !first.GetCreated() || first.GetNodeId() != matched.GetNodeId() ||
		first.GetLiveContent() == "" || len(first.GetRetrievedSources()) != 1 {
		t.Fatalf("first promotion=%+v", first)
	}
	if !strings.HasPrefix(first.GetSeedPath(), backend.seedsDir) {
		t.Fatalf("seed path %q outside %q", first.GetSeedPath(), backend.seedsDir)
	}
	replay := promote()
	if replay.GetCreated() || replay.GetNodeId() != first.GetNodeId() {
		t.Fatalf("replay=%+v", replay)
	}

	node := httptest.NewRecorder()
	gw.Handler().ServeHTTP(node, httptest.NewRequest(http.MethodGet, "/api/nodes/"+first.GetNodeId(), nil))
	if node.Code != http.StatusOK || !strings.Contains(node.Body.String(), "stage_markdown_default") {
		t.Fatalf("promoted node status=%d body=%s", node.Code, node.Body.String())
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

	rr3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodGet, "/js/app.js", nil)
	gw.Handler().ServeHTTP(rr3, req3)
	if got := rr3.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("/js/app.js Cache-Control: want no-cache, got %q", got)
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

func startSecureGateway(t *testing.T, teacherKey string) (*webgateway.Gateway, *testBackend, func()) {
	t.Helper()
	t.Setenv("AVLP_SESSION_SECRET", "test-session-secret-32chars-min!!")
	if teacherKey != "" {
		t.Setenv("AVLP_TEACHER_KEY", teacherKey)
	} else {
		t.Setenv("AVLP_TEACHER_KEY", "")
	}
	gw, backend, cleanup := startTestGateway(t)
	gw.Session = session.FromEnv()
	return gw, backend, cleanup
}

func sessionToken(t *testing.T, gw *webgateway.Gateway, studentID, teacherKey string) string {
	t.Helper()
	body := map[string]string{"student_id": studentID}
	if teacherKey != "" {
		body["teacher_key"] = teacherKey
	}
	raw, _ := json.Marshal(body)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/session", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	gw.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("session status=%d body=%s", rr.Code, rr.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	tok, _ := out["token"].(string)
	if tok == "" || out["secure_mode"] != true {
		t.Fatalf("session response: %v", out)
	}
	return tok
}

func TestGatewaySecureIDORStationNotFound(t *testing.T) {
	gw, backend, cleanup := startSecureGateway(t, "")
	defer cleanup()
	backend.live.fail = true

	tokA := sessionToken(t, gw, "stu-a", "")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/query",
		strings.NewReader(`{"student_id":"stu-a","query_text":"duda novel auth"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokA)
	gw.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("query status=%d body=%s", rr.Code, rr.Body.String())
	}
	var route map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &route)
	pending, _ := route["pending"].(map[string]any)
	ulid, _ := pending["tracking_ulid"].(string)
	if ulid == "" {
		t.Fatalf("expected pending tracking, got %s", rr.Body.String())
	}

	backend.live.fail = false
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet,
		"/api/stations/"+ulid+"?student_id=stu-a", nil)
	req.Header.Set("Authorization", "Bearer "+tokA)
	gw.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("owner poll status=%d body=%s", rr.Code, rr.Body.String())
	}

	tokB := sessionToken(t, gw, "stu-b", "")
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet,
		"/api/stations/"+ulid+"?student_id=stu-a", nil)
	req.Header.Set("Authorization", "Bearer "+tokB)
	gw.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("IDOR want 404, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestGatewaySecurePromoteRequiresTeacher(t *testing.T) {
	gw, backend, cleanup := startSecureGateway(t, "institute-key")
	defer cleanup()
	backend.live.fail = true

	tokStudent := sessionToken(t, gw, "stu-p", "")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/query",
		strings.NewReader(`{"student_id":"stu-p","query_text":"promover auth"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokStudent)
	gw.Handler().ServeHTTP(rr, req)
	var route map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &route)
	pending, _ := route["pending"].(map[string]any)
	ulid, _ := pending["tracking_ulid"].(string)
	if ulid == "" {
		t.Fatalf("no tracking: %s", rr.Body.String())
	}

	backend.live.fail = false
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet,
		"/api/stations/"+ulid+"?student_id=stu-p", nil)
	req.Header.Set("Authorization", "Bearer "+tokStudent)
	gw.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("poll ready status=%d body=%s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/stations/"+ulid+"/promote", nil)
	req.Header.Set("Authorization", "Bearer "+tokStudent)
	gw.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("student promote want 403, got %d body=%s", rr.Code, rr.Body.String())
	}

	tokTeacher := sessionToken(t, gw, "stu-p", "institute-key")
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/stations/"+ulid+"/promote", nil)
	req.Header.Set("Authorization", "Bearer "+tokTeacher)
	gw.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("teacher promote status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestGatewayOpenModeSession(t *testing.T) {
	t.Setenv("AVLP_SESSION_SECRET", "")
	gw, _, cleanup := startTestGateway(t)
	defer cleanup()
	gw.Session = session.FromEnv()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/session",
		strings.NewReader(`{"student_id":"stu-open"}`))
	req.Header.Set("Content-Type", "application/json")
	gw.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var out map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	if out["secure_mode"] != false {
		t.Fatalf("expected open mode: %v", out)
	}
	if out["stt_enabled"] != false {
		t.Fatalf("expected stt disabled by default: %v", out)
	}
}

func TestGatewayTranscribeUnavailableWithoutSTT(t *testing.T) {
	gw, _, cleanup := startTestGateway(t)
	defer cleanup()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/transcribe", strings.NewReader("not-multipart"))
	req.Header.Set("Content-Type", "text/plain")
	gw.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestGatewayTranscribeSuccess(t *testing.T) {
	sttSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/audio/transcriptions" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"text":"dictado de prueba"}`))
	}))
	t.Cleanup(sttSrv.Close)

	tr, err := stt.NewHTTPTranscriber(stt.HTTPTranscriberConfig{URL: sttSrv.URL + "/v1"})
	if err != nil {
		t.Fatal(err)
	}
	gw, _, cleanup := startTestGateway(t)
	defer cleanup()
	gw.Transcriber = tr

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/session",
		strings.NewReader(`{"student_id":"stu-stt"}`))
	req.Header.Set("Content-Type", "application/json")
	gw.Handler().ServeHTTP(rr, req)
	var sess map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &sess)
	if sess["stt_enabled"] != true {
		t.Fatalf("stt_enabled=%v", sess["stt_enabled"])
	}

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("file", "clip.webm")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("fake-audio-bytes"))
	_ = w.Close()

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/transcribe", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	gw.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var out map[string]string
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	if out["text"] != "dictado de prueba" {
		t.Fatalf("out=%v", out)
	}
}

