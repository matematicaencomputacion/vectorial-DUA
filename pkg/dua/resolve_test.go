package dua_test

import (
	"testing"

	"github.com/vectorial-dua/avlp/pkg/dua"
)

func TestResolveRoutingHints_ExplicitFrustrationZeroNotOverwritten(t *testing.T) {
	dim, format, fr := dua.ResolveRoutingHints(dua.ResolveRoutingHintsInput{
		Dimensions:          []float32{0.4, 0.4, 0.9, 0.4, 0.4},
		FrustrationSignal:   0.0,
		FrustrationProvided: true, // explicit calm signal
	})
	if fr != 0.0 {
		t.Fatalf("frustration=%v want 0.0", fr)
	}
	if dim != string(dua.Representacion) {
		t.Fatalf("dimension=%s want=%s", dim, dua.Representacion)
	}
	if format != string(dua.Conceptual) {
		t.Fatalf("format=%s want=%s", format, dua.Conceptual)
	}
}

func TestResolveRoutingHints_InferDimensionAccionFromFrustration(t *testing.T) {
	dim, _, _ := dua.ResolveRoutingHints(dua.ResolveRoutingHintsInput{
		Dimensions:          []float32{0.3, 0.3, 0.8, 0.2, 0.3},
		FrustrationProvided: false,
	})
	if dim != string(dua.Accion) {
		t.Fatalf("dimension=%s want=%s", dim, dua.Accion)
	}
}

func TestResolveRoutingHints_InferDimensionCompromisoFromAutonomia(t *testing.T) {
	dim, _, _ := dua.ResolveRoutingHints(dua.ResolveRoutingHintsInput{
		Dimensions:          []float32{0.4, 0.4, 0.2, 0.4, 0.9},
		FrustrationProvided: false,
	})
	if dim != string(dua.Compromiso) {
		t.Fatalf("dimension=%s want=%s", dim, dua.Compromiso)
	}
}

func TestResolveRoutingHints_FrustrationOverridesAutonomia(t *testing.T) {
	dim, _, _ := dua.ResolveRoutingHints(dua.ResolveRoutingHintsInput{
		Dimensions:          []float32{0.4, 0.4, 0.9, 0.4, 0.8},
		FrustrationProvided: false,
	})
	if dim != string(dua.Accion) {
		t.Fatalf("dimension=%s want=%s", dim, dua.Accion)
	}
}

func TestResolveRoutingHints_InferDimensionRepresentacionDefault(t *testing.T) {
	dim, _, _ := dua.ResolveRoutingHints(dua.ResolveRoutingHintsInput{
		Dimensions:          []float32{0.4, 0.4, 0.3, 0.4, 0.4},
		FrustrationProvided: false,
	})
	if dim != string(dua.Representacion) {
		t.Fatalf("dimension=%s want=%s", dim, dua.Representacion)
	}
}

func TestResolveRoutingHints_RespectsPreferredDimensionAndFormat(t *testing.T) {
	dim, format, _ := dua.ResolveRoutingHints(dua.ResolveRoutingHintsInput{
		Dimensions:         []float32{0.9, 0.9, 0.9, 0.9, 0.9},
		PreferredDimension: string(dua.Accion),
		PreferredFormat:    string(dua.Visual),
	})
	if dim != string(dua.Accion) || format != string(dua.Visual) {
		t.Fatalf("got dim=%s format=%s", dim, format)
	}
}
