package knowledge_test

import (
	"path/filepath"
	"testing"

	"github.com/vectorial-dua/avlp/pkg/knowledge"
)

func TestLoadProductionCurriculum(t *testing.T) {
	path := filepath.Join("..", "..", "data", "knowledge", "curriculum.json")
	g, rep, err := knowledge.LoadFile(path, knowledge.LoadOptions{})
	if err != nil {
		t.Fatalf("LoadFile production curriculum: %v", err)
	}
	n := len(g.ConceptIDs())
	if n < 15 || n > 25 {
		t.Fatalf("concept count %d outside 15–25", n)
	}
	if len(g.Edges()) == 0 {
		t.Fatal("expected edges")
	}
	_ = rep
}
