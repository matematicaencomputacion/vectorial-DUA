package dua_test

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/vectorial-dua/avlp/pkg/dua"
	"github.com/vectorial-dua/avlp/pkg/rag"
	"github.com/vectorial-dua/avlp/pkg/vector"
)

func seedDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "data", "nodes", "interactive"))
}

func kbDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "data", "knowledge_base"))
}

func TestLoadAndValidateSeed(t *testing.T) {
	reg := dua.NewRegistry()
	n, err := reg.LoadDir(seedDir(t))
	if err != nil {
		t.Fatal(err)
	}
	if n < 1 {
		t.Fatalf("expected seeds, got %d", n)
	}
	node, ok := reg.Get("dua::Representacion::basico::visual::01ARZ3NDEKTSV4RRFFQ69G5FAV")
	if !ok {
		t.Fatal("seed node missing")
	}
	if node.BotoneraSchema == nil || node.BotoneraSchema.Kind != dua.SchemaDepth {
		t.Fatalf("expected depth botonera_schema, got %+v", node.BotoneraSchema)
	}
	if len(node.BotoneraSchema.DepthOptions) < 2 {
		t.Fatalf("depth_options too small: %d", len(node.BotoneraSchema.DepthOptions))
	}
}

func TestValidateRejectsEmptyBotonera(t *testing.T) {
	n := &dua.InteractiveVideoNode{
		NodeID:            "dua::Representacion::basico::visual::01ARZ3NDEKTSV4RRFFQ69G5FAV",
		DimensionDUA:      "Representacion",
		Titulo:            "x",
		LayoutType:        dua.LayoutInteractiveDashboard,
		StageMediaDefault: "https://cdn.example.com/a.mp4",
		Botonera:          nil,
	}
	if err := n.Validate(); err == nil {
		t.Fatal("expected error")
	}
}

func TestMutateAppendsLiveButton(t *testing.T) {
	reg := dua.NewRegistry()
	if _, err := reg.LoadDir(seedDir(t)); err != nil {
		t.Fatal(err)
	}
	store := rag.NewStore()
	emb := rag.DefaultEmbedder()
	if _, err := rag.IngestWalk(context.Background(), store, rag.IngestOptions{Root: kbDir(t), Embedder: emb}); err != nil {
		t.Fatal(err)
	}
	m := &dua.Mutator{
		Registry:  reg,
		Retriever: rag.NewRetriever(store, emb, 3),
	}
	nodeID := "dua::Representacion::basico::visual::01ARZ3NDEKTSV4RRFFQ69G5FAV"
	before, _ := reg.Get(nodeID)
	res, err := m.Mutate(context.Background(), dua.MutateRequest{
		NodeID:      nodeID,
		StudentID:   "stu-1",
		DoubtText:   "No entiendo el scope de una variable let en un bloque",
		Frustration: 0.7,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Button.IsLiveGenerated {
		t.Fatal("expected is_live_generated")
	}
	// El label es student-facing: la duda, nunca el archivo que la ancló.
	if strings.ContainsAny(res.Button.Label, "[]") {
		t.Fatalf("label expone material interno: %q", res.Button.Label)
	}
	if len(res.Button.VectorDelta) != vector.ContentEmbedDims {
		t.Fatalf("vector_delta dims=%d want content space %d (not V_e)", len(res.Button.VectorDelta), vector.ContentEmbedDims)
	}
	if len(res.Node.Botonera) != len(before.Botonera)+1 {
		t.Fatalf("botonera size=%d want %d", len(res.Node.Botonera), len(before.Botonera)+1)
	}
	after, _ := reg.Get(nodeID)
	if len(after.Botonera) != len(before.Botonera)+1 {
		t.Fatal("registry not updated")
	}
}
