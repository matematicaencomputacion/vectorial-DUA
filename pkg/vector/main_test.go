package vector_test

import (
	"os"
	"testing"

	"github.com/vectorial-dua/avlp/internal/testenv"
)

// Routing thresholds, station TTL and the RAG toggle come from AVLP_*; clear
// them so results depend on the code under test, not on the operator's shell.
// Tests that need a specific value still set it with t.Setenv.
func TestMain(m *testing.M) {
	testenv.Clear()
	os.Exit(m.Run())
}
