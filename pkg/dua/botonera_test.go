package dua_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/vectorial-dua/avlp/pkg/dua"
)

func TestBotoneraSchemasLoad(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "data", "nodes", "interactive"))
	reg := dua.NewRegistry()
	n, err := reg.LoadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if n < 4 {
		t.Fatalf("expected >=4 seeds, got %d", n)
	}

	depth, ok := reg.Get("dua::Representacion::basico::visual::01ARZ3NDEKTSV4RRFFQ69G5FAV")
	if !ok || depth.BotoneraSchema == nil || depth.BotoneraSchema.Kind != dua.SchemaDepth {
		t.Fatalf("depth seed missing/invalid: %+v", depth)
	}

	cog, ok := reg.Get("dua::Representacion::basico::visual::01ARZ3NDEKTSV4RRFFQ69G5FB0")
	if !ok || cog.BotoneraSchema.Kind != dua.SchemaCognitive {
		t.Fatal("cognitive seed missing")
	}

	em, ok := reg.Get("dua::Accion::basico::practica::01ARZ3NDEKTSV4RRFFQ69G5FB1")
	if !ok || em.BotoneraSchema.Kind != dua.SchemaEmergency {
		t.Fatal("emergency seed missing")
	}
	for _, opt := range em.BotoneraSchema.EmergencyOptions {
		if opt.VariantID == "hint" && opt.HintText == "" {
			t.Fatal("hint without hint_text")
		}
	}

	comb, ok := reg.Get("dua::Representacion::basico::visual::01ARZ3NDEKTSV4RRFFQ69G5FB2")
	if !ok || comb.BotoneraSchema.Kind != dua.SchemaCombined {
		t.Fatal("combined seed missing")
	}
	cell, found := comb.BotoneraSchema.ResolveCombined("express", "video")
	if !found || cell.MediaURL == "" {
		t.Fatalf("expected express×video cell, got %+v", cell)
	}
}

func TestHintRequiresText(t *testing.T) {
	b := &dua.DUANodeBotonera{
		Kind: dua.SchemaEmergency,
		EmergencyOptions: []dua.EmergencyVariant{{
			VariantID:  "hint",
			Label:      "Hint",
			FormatType: dua.MediaTextHint,
		}},
	}
	if err := b.Validate(); err == nil {
		t.Fatal("expected hint_text error")
	}
}
