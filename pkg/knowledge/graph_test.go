package knowledge_test

import (
	"path/filepath"
	"testing"

	"github.com/vectorial-dua/avlp/pkg/knowledge"
)

func fixturePath(t *testing.T) string {
	t.Helper()
	return filepath.Join("testdata", "curriculum.json")
}

func TestMemoryGraphDeterminismAndDirection(t *testing.T) {
	g, _, err := knowledge.LoadFile(fixturePath(t), knowledge.LoadOptions{})
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}

	gamma := knowledge.ConceptID("concept:gamma")
	prereq := g.Prerequisites(gamma)
	// strength desc: beta(0.8) before alpha(0.5)
	if len(prereq) != 2 || prereq[0] != "concept:beta" || prereq[1] != "concept:alpha" {
		t.Fatalf("Prerequisites(gamma)=%v", prereq)
	}

	deps := g.Dependents(knowledge.ConceptID("concept:alpha"))
	// beta and gamma require alpha (via edges); order by incoming edge strength then from id
	if len(deps) != 2 {
		t.Fatalf("Dependents(alpha)=%v", deps)
	}
	if deps[0] != "concept:beta" || deps[1] != "concept:gamma" {
		// beta strength 0.9 > gamma strength 0.5
		t.Fatalf("Dependents(alpha) order=%v want [beta, gamma]", deps)
	}

	for _, id := range g.ConceptIDs() {
		p := map[knowledge.ConceptID]struct{}{}
		for _, x := range g.Prerequisites(id) {
			p[x] = struct{}{}
		}
		for _, x := range g.Dependents(id) {
			if _, ok := p[x]; ok {
				t.Fatalf("direction invariant violated for %s: %s in both Prerequisites and Dependents", id, x)
			}
		}
	}

	path, ok := g.Path("concept:gamma", "concept:alpha")
	if !ok {
		t.Fatal("Path gamma→alpha expected")
	}
	// learning order: foundational first
	if len(path) < 2 || path[0] != "concept:alpha" || path[len(path)-1] != "concept:gamma" {
		t.Fatalf("Path learning order=%v", path)
	}

	// Neighbors alternative is undirected
	alts := g.Neighbors("concept:gamma", knowledge.EdgeAlternative)
	if len(alts) != 1 || alts[0].To != "concept:epsilon" {
		t.Fatalf("Neighbors alternative=%v", alts)
	}
	alts2 := g.Neighbors("concept:epsilon", knowledge.EdgeAlternative)
	if len(alts2) != 1 || alts2[0].To != "concept:gamma" {
		t.Fatalf("Neighbors alternative reverse=%v", alts2)
	}

	// Second load must produce identical sequences
	g2, _, err := knowledge.LoadFile(fixturePath(t), knowledge.LoadOptions{})
	if err != nil {
		t.Fatalf("LoadFile2: %v", err)
	}
	got1 := g.Prerequisites(gamma)
	got2 := g2.Prerequisites(gamma)
	if len(got1) != len(got2) {
		t.Fatalf("determinism len %d vs %d", len(got1), len(got2))
	}
	for i := range got1 {
		if got1[i] != got2[i] {
			t.Fatalf("determinism mismatch at %d: %s vs %s", i, got1[i], got2[i])
		}
	}
}
