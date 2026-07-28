package dua_test

import (
	"testing"

	"github.com/vectorial-dua/avlp/pkg/dua"
)

func TestHierarchySeedLoadAndPath(t *testing.T) {
	reg := dua.NewRegistry()
	if _, err := reg.LoadDir(seedDir(t)); err != nil {
		t.Fatal(err)
	}
	node, ok := reg.Get("dua::Representacion::basico::visual::01ARZ3NDEKTSV4RRFFQ69G5FC0")
	if !ok {
		t.Fatal("automovil hierarchy seed missing")
	}
	if node.Hierarchy == nil {
		t.Fatal("expected hierarchy")
	}
	if err := node.Hierarchy.Validate(); err != nil {
		t.Fatal(err)
	}
	// Non-linear: open Motor without Asientos.
	motor, found := node.Hierarchy.FindByID("sub_motor")
	if !found || motor.Title != "Motor" {
		t.Fatalf("FindByID motor: found=%v node=%+v", found, motor)
	}
	path, ok := node.Hierarchy.PathTo("sub_motor")
	if !ok {
		t.Fatal("PathTo motor failed")
	}
	if len(path) != 2 || path[0] != "sub_caja_central" || path[1] != "sub_motor" {
		t.Fatalf("unexpected path: %v", path)
	}
	proto := dua.ToProto(node)
	if proto.GetHierarchy() == nil || len(proto.GetHierarchy().GetSubtopics()) != 2 {
		t.Fatalf("ToProto hierarchy: %+v", proto.GetHierarchy())
	}
}

func TestHierarchyRejectsDuplicateIDs(t *testing.T) {
	tree := &dua.DUAHierarchicalTree{
		MainTopicTitle: "X",
		MacroMediaURL:  "https://cdn.example.com/m.mp4",
		Subtopics: []dua.SubtopicNode{
			{
				SubtopicID: "a",
				Title:      "A",
				DepthLevel: dua.DepthComponent,
				IsOptional: true,
				MediaURL:   "https://cdn.example.com/a.mp4",
				ChildSubtopics: []dua.SubtopicNode{
					{SubtopicID: "a", Title: "dup", DepthLevel: dua.DepthMicro, IsOptional: true, MediaURL: "https://cdn.example.com/b.mp4"},
				},
			},
		},
	}
	if err := tree.Validate(); err == nil {
		t.Fatal("expected duplicate id error")
	}
}

func TestInteractionStoreRecord(t *testing.T) {
	store := dua.NewInteractionStore()
	store.Record("stu-1", "node-1", "sub_motor", []float32{0.1})
	if !store.HasOpened("stu-1", "node-1", "sub_motor") {
		t.Fatal("expected HasOpened")
	}
	if store.HasOpened("stu-1", "node-1", "sub_asientos") {
		t.Fatal("should not have opened asientos")
	}
	list := store.OpenedList("stu-1", "node-1")
	if len(list) != 1 || list[0] != "sub_motor" {
		t.Fatalf("OpenedList=%v", list)
	}
}

func TestInteractionStoreRecordAppliesOrbitDeltaToProfile(t *testing.T) {
	reg := dua.NewRegistry()
	if _, err := reg.LoadDir(seedDir(t)); err != nil {
		t.Fatal(err)
	}
	node, ok := reg.Get("dua::Representacion::basico::visual::01ARZ3NDEKTSV4RRFFQ69G5FC0")
	if !ok || node.Hierarchy == nil {
		t.Fatal("hierarchy seed missing")
	}
	delta := dua.ResolveSubtopicDelta(node.Hierarchy, "sub_motor", nil)
	if len(delta) != dua.VeDims {
		t.Fatalf("delta dims=%d want=%d", len(delta), dua.VeDims)
	}

	profiles := dua.NewProfileStore()
	store := dua.NewInteractionStoreWithProfiles(profiles)

	store.Record("stu-2", "node-2", "sub_motor", delta)
	if !store.HasOpened("stu-2", "node-2", "sub_motor") {
		t.Fatal("expected touch tracking")
	}
	cur := profiles.Get("stu-2")
	if cur[0] <= dua.DefaultVe()[0] {
		t.Fatalf("expected profile update on valid delta, got %v", cur)
	}
}

func TestInteractionStoreRecordInvalidDeltaStillTracksTouch(t *testing.T) {
	profiles := dua.NewProfileStore()
	store := dua.NewInteractionStoreWithProfiles(profiles)

	before := profiles.Get("stu-3")
	store.Record("stu-3", "node-3", "sub_bad", []float32{0.3}) // wrong dims

	if !store.HasOpened("stu-3", "node-3", "sub_bad") {
		t.Fatal("touch tracking must complete even on bad delta")
	}
	after := profiles.Get("stu-3")
	for i := range before {
		if before[i] != after[i] {
			t.Fatalf("profile should be unchanged at %d: before=%v after=%v", i, before, after)
		}
	}
}

func TestValidateAllowsHierarchyOnly(t *testing.T) {
	n := &dua.InteractiveVideoNode{
		NodeID:            "dua::Representacion::basico::visual::01ARZ3NDEKTSV4RRFFQ69G5FC0",
		DimensionDUA:      "Representacion",
		Titulo:            "Auto",
		LayoutType:        dua.LayoutInteractiveDashboard,
		StageMediaDefault: "https://cdn.example.com/m.mp4",
		Hierarchy: &dua.DUAHierarchicalTree{
			MainTopicTitle: "Auto",
			MacroMediaURL:  "https://cdn.example.com/m.mp4",
			Subtopics: []dua.SubtopicNode{
				{
					SubtopicID: "ruedas",
					Title:      "Ruedas",
					DepthLevel: dua.DepthComponent,
					IsOptional: true,
					MediaURL:   "https://cdn.example.com/r.mp4",
				},
			},
		},
	}
	if err := n.Validate(); err != nil {
		t.Fatal(err)
	}
}
