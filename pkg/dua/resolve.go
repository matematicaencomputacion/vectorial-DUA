package dua

// ResolveRoutingHintsInput carries request/profile hints used by router miss-path
// scaffolding and live-station generation.
type ResolveRoutingHintsInput struct {
	Dimensions          []float32
	PreferredDimension  string
	PreferredFormat     string
	FrustrationSignal   float32
	FrustrationProvided bool
}

// ResolveRoutingHints normalizes dimension/format/frustration using explicit
// request hints first, then profile-derived fallbacks.
func ResolveRoutingHints(in ResolveRoutingHintsInput) (dimension, format string, frustration float32) {
	frustration = in.FrustrationSignal
	if !in.FrustrationProvided {
		frustration = frustrationFromDims(in.Dimensions)
	}

	dimension = in.PreferredDimension
	if !ValidDimension(dimension) {
		dimension = inferDimension(in.Dimensions, frustration)
	}

	format = in.PreferredFormat
	if format == "" {
		format = inferFormat(in.Dimensions)
	}
	return dimension, format, frustration
}

func inferDimension(dims []float32, frustration float32) string {
	autonomia := dimAt(dims, 4)
	if autonomia >= 0.70 {
		return string(Compromiso)
	}
	if frustration >= 0.65 {
		return string(Accion)
	}
	return string(Representacion)
}

func inferFormat(dims []float32) string {
	sensorial := dimAt(dims, 1)
	dominio := dimAt(dims, 0)
	autonomia := dimAt(dims, 4)
	if autonomia >= 0.70 && dominio >= 0.60 {
		return string(Practica)
	}
	if sensorial >= 0.65 {
		return string(Visual)
	}
	return string(Conceptual)
}

func frustrationFromDims(dims []float32) float32 {
	return dimAt(dims, 2)
}

func dimAt(dims []float32, idx int) float32 {
	if idx < 0 || idx >= len(dims) {
		return 0
	}
	return dims[idx]
}
