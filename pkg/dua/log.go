package dua

// Logf is an optional logger hook for pkg/dua.
// nil means silent (no output) — preferred for unit tests.
// cmd/router injects log.Printf for operational visibility.
type Logf func(format string, args ...any)

func (f Logf) printf(format string, args ...any) {
	if f != nil {
		f(format, args...)
	}
}
