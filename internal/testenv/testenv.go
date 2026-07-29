// Package testenv keeps tests independent from the shell that launches them.
//
// Routing reads AVLP_* variables at run time (similarity threshold, embedder
// URL, RAG floors, interactive-node toggles). An operator with
// AVLP_SIMILARITY_THRESHOLD=0.55 exported would otherwise see different golden
// outcomes than CI, which once masked a real regression.
package testenv

import (
	"os"
	"strings"
	"testing"
)

// Prefix marks the variables that configure AVLP at run time.
const Prefix = "AVLP_"

// Isolate unsets every AVLP_* variable for the duration of the test and
// restores the previous values on cleanup. Tests that need a specific value
// call t.Setenv after this.
func Isolate(t testing.TB) {
	t.Helper()
	for _, key := range keys() {
		// t.Setenv registers the restore; Unsetenv then makes absence
		// unambiguous for code that distinguishes empty from unset.
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("testenv: unset %s: %v", key, err)
		}
	}
}

// Clear unsets every AVLP_* variable process-wide, for use from TestMain when a
// whole package depends on routing defaults.
func Clear() {
	for _, key := range keys() {
		_ = os.Unsetenv(key)
	}
}

func keys() []string {
	var out []string
	for _, entry := range os.Environ() {
		key, _, ok := strings.Cut(entry, "=")
		if ok && strings.HasPrefix(key, Prefix) {
			out = append(out, key)
		}
	}
	return out
}
