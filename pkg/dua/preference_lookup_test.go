package dua_test

import (
	"testing"

	"github.com/vectorial-dua/avlp/pkg/dua"
)

func TestResolveBotoneraDeltaFromEmergencySeedHint(t *testing.T) {
	reg := dua.NewRegistry()
	if _, err := reg.LoadDir(seedDir(t)); err != nil {
		t.Fatal(err)
	}
	node, ok := reg.Get("dua::Accion::basico::practica::01ARZ3NDEKTSV4RRFFQ69G5FB1")
	if !ok || node.BotoneraSchema == nil {
		t.Fatal("emergency seed missing")
	}

	delta := dua.ResolveBotoneraDelta(node.BotoneraSchema, "hint", nil)
	if len(delta) != dua.VeDims {
		t.Fatalf("delta dims=%d want %d", len(delta), dua.VeDims)
	}
	want := []float32{0.0, 0.0, -0.3, 0.0, 0.2}
	for i := range want {
		if delta[i] != want[i] {
			t.Fatalf("delta[%d]=%v want=%v (full=%v)", i, delta[i], want[i], delta)
		}
	}
}

func TestResolveBotoneraDeltaPrefersClientDelta(t *testing.T) {
	client := []float32{0.1, 0.2, 0.3, 0.4, 0.5}
	got := dua.ResolveBotoneraDelta(nil, "hint", client)
	if len(got) != len(client) {
		t.Fatalf("len=%d want=%d", len(got), len(client))
	}
	for i := range client {
		if got[i] != client[i] {
			t.Fatalf("got[%d]=%v want=%v", i, got[i], client[i])
		}
	}
}

func TestResolveSubtopicDeltaFromTreeOrbitDelta(t *testing.T) {
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
	want := []float32{0.08, 0.0, 0.0, 0.05, 0.0}
	for i := range want {
		if delta[i] != want[i] {
			t.Fatalf("delta[%d]=%v want=%v (full=%v)", i, delta[i], want[i], delta)
		}
	}
}
