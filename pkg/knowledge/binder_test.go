package knowledge_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vectorial-dua/avlp/pkg/dua"
	"github.com/vectorial-dua/avlp/pkg/knowledge"
	"github.com/vectorial-dua/avlp/pkg/vector"
)

func TestIndexBinder(t *testing.T) {
	ctx := context.Background()
	idx := vector.NewIndexWithDims(4)
	if err := idx.Upsert(vector.Node{
		ID:          "dua::Representacion::basico::visual::01ARZ3NDEKTSV4RRFFQ69G5FZZ",
		ResourceURL: "master://nodes/env-diagram",
		Embedding:   []float32{1, 0, 0, 0},
		Concepts:    []string{"concept:alpha", "env-file-missing"},
	}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if err := idx.Upsert(vector.Node{
		ID:              "dua::Representacion::basico::visual::01ARZ3NDEKTSV4RRFFQ69G5FZY",
		ResourceURL:     "live://x",
		Embedding:       []float32{0, 1, 0, 0},
		IsLiveGenerated: true,
		Concepts:        []string{"concept:alpha"},
	}); err != nil {
		t.Fatalf("Upsert live: %v", err)
	}

	reg := dua.NewRegistry()
	if err := reg.Put(&dua.InteractiveVideoNode{
		NodeID:            "interactive-1",
		DimensionDUA:      "Representacion",
		Titulo:            "Demo",
		LayoutType:        dua.LayoutInteractiveDashboard,
		StageMediaDefault: "https://cdn.example.com/demo.mp4",
		Concepts:          []string{"concept:beta"},
		Embedding:         []float32{0, 0, 1, 0},
		Botonera: []dua.InteractiveButton{{
			IDBtn:      "ask",
			Label:      "Preguntar",
			ActionType: dua.ActionAskAgent,
		}},
	}); err != nil {
		t.Fatalf("Put: %v", err)
	}

	binder := &knowledge.IndexBinder{Index: idx, Registry: reg}
	fixture := filepath.Join("testdata", "curriculum.json")
	g, rep, err := knowledge.LoadFile(fixture, knowledge.LoadOptions{Binder: binder})
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}

	resAlpha, err := binder.ResourcesFor(ctx, "concept:alpha")
	if err != nil {
		t.Fatalf("ResourcesFor: %v", err)
	}
	if len(resAlpha) != 1 || resAlpha[0] != "master://nodes/env-diagram" {
		t.Fatalf("ResourcesFor alpha=%v (live should be skipped)", resAlpha)
	}
	ids, err := binder.ConceptsForNode(ctx, "interactive-1")
	if err != nil {
		t.Fatalf("ConceptsForNode: %v", err)
	}
	if len(ids) != 1 || ids[0] != "concept:beta" {
		t.Fatalf("ConceptsForNode=%v", ids)
	}

	foundAbsent := false
	foundUntaught := false
	for _, w := range rep.Warnings {
		if strings.Contains(w, "resource declares absent concept: concept:env-file-missing") {
			foundAbsent = true
		}
		if strings.Contains(w, "concept without teaching resource: concept:gamma") {
			foundUntaught = true
		}
	}
	if !foundAbsent {
		t.Fatalf("expected absent-concept warning, got %#v", rep.Warnings)
	}
	if !foundUntaught {
		t.Fatalf("expected untaught warning, got %#v", rep.Warnings)
	}

	t.Setenv("AVLP_KNOWLEDGE_STRICT", "true")
	if !knowledge.StrictFromEnv() {
		t.Fatal("StrictFromEnv should be true")
	}
	_, _, err = knowledge.LoadFile(fixture, knowledge.LoadOptions{
		Strict: true,
		Binder: &knowledge.IndexBinder{Index: idx, Registry: reg},
	})
	if err == nil {
		t.Fatal("expected strict binding error")
	}

	st := g.Stats()
	if st.Concepts != 5 || st.Edges != 6 {
		t.Fatalf("Stats size concepts=%d edges=%d", st.Concepts, st.Edges)
	}
	if err := g.Health(ctx); err != nil {
		t.Fatalf("Health: %v", err)
	}
}
