package knowledge_test

import (
	"os"
	"testing"

	"github.com/vectorial-dua/avlp/internal/testenv"
)

func TestMain(m *testing.M) {
	testenv.Clear()
	os.Exit(m.Run())
}
