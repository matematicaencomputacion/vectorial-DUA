// Package dua provides DUA dimension helpers for the adaptive learning manifold.
// Full Master journey mutation lands in a later OpenSpec change.
package dua

// Dimension enumerates Universal Design for Learning axes.
type Dimension string

const (
	Representacion Dimension = "Representacion" // What
	Accion         Dimension = "Accion"         // How
	Compromiso     Dimension = "Compromiso"     // Why
)

// Format is a sensory/expression modality for a node.
type Format string

const (
	Visual     Format = "visual"
	Conceptual Format = "conceptual"
	Practica   Format = "practica"
)

// ValidDimension reports whether d is a known DUA axis.
func ValidDimension(d string) bool {
	switch Dimension(d) {
	case Representacion, Accion, Compromiso:
		return true
	default:
		return false
	}
}
