package dua_test

import (
	"testing"

	"github.com/vectorial-dua/avlp/pkg/dua"
)

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
