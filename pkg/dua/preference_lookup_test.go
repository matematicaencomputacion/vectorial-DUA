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

	delta := dua.ResolveBotoneraDelta(node, "hint", nil)
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

func TestHasBotoneraVariantEmergencySeed(t *testing.T) {
	reg := dua.NewRegistry()
	if _, err := reg.LoadDir(seedDir(t)); err != nil {
		t.Fatal(err)
	}
	node, ok := reg.Get("dua::Accion::basico::practica::01ARZ3NDEKTSV4RRFFQ69G5FB1")
	if !ok || node.BotoneraSchema == nil {
		t.Fatal("emergency seed missing")
	}
	if !dua.HasBotoneraVariant(node, "hint", "") {
		t.Fatal("expected hint variant")
	}
	if dua.HasBotoneraVariant(node, "missing", "") {
		t.Fatal("unexpected variant")
	}
}

func TestHasBotoneraVariantCombinedSeed(t *testing.T) {
	reg := dua.NewRegistry()
	if _, err := reg.LoadDir(seedDir(t)); err != nil {
		t.Fatal(err)
	}
	node, ok := reg.Get("dua::Representacion::basico::visual::01ARZ3NDEKTSV4RRFFQ69G5FB2")
	if !ok || node.BotoneraSchema == nil {
		t.Fatal("combined seed missing")
	}
	if !dua.HasBotoneraVariant(node, "express", "video") {
		t.Fatal("expected express×video cell")
	}
	if dua.HasBotoneraVariant(node, "express", "") {
		t.Fatal("combined requires format_id")
	}
}

func TestLegacyBotoneraAskDifferentVariablesScope(t *testing.T) {
	reg := dua.NewRegistry()
	if _, err := reg.LoadDir(seedDir(t)); err != nil {
		t.Fatal(err)
	}
	node, ok := reg.Get("dua::Representacion::basico::visual::01ARZ3NDEKTSV4RRFFQ69G5FAV")
	if !ok {
		t.Fatal("variables-scope seed missing")
	}
	if !dua.HasBotoneraVariant(node, "ask_different", "") {
		t.Fatal("expected legacy id_btn ask_different")
	}

	// Without client preference_delta: Ack-path empty — never apply vector_delta to V_e.
	if delta := dua.ResolveBotoneraDelta(node, "ask_different", nil); len(delta) != 0 {
		t.Fatalf("legacy without client delta must be empty, got %v", delta)
	}

	client := []float32{0.05, 0, 0, 0, 0.1}
	got := dua.ResolveBotoneraDelta(node, "ask_different", client)
	for i := range client {
		if got[i] != client[i] {
			t.Fatalf("client delta[%d]=%v want=%v", i, got[i], client[i])
		}
	}

	// Legacy-only node (no schema): existence + empty delta without client.
	legacyOnly := &dua.InteractiveVideoNode{
		Botonera: []dua.InteractiveButton{{IDBtn: "ask_different", VectorDelta: []float32{0, 0, 0.2, 0, 0.3}}},
	}
	if !dua.HasBotoneraVariant(legacyOnly, "ask_different", "") {
		t.Fatal("expected legacy-only match")
	}
	if delta := dua.ResolveBotoneraDelta(legacyOnly, "ask_different", nil); len(delta) != 0 {
		t.Fatalf("must not promote vector_delta to V_e, got %v", delta)
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
