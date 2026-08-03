package knowledge_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/vectorial-dua/avlp/pkg/knowledge"
)

func fixturePath(t *testing.T) string {
	t.Helper()
	return filepath.Join("testdata", "curriculum.json")
}

func TestMemoryGraphDeterminismAndDirection(t *testing.T) {
	ctx := context.Background()
	g, _, err := knowledge.LoadFile(fixturePath(t), knowledge.LoadOptions{})
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}

	gamma := knowledge.ConceptID("concept:gamma")
	prereq, err := g.Prerequisites(ctx, gamma, knowledge.TraverseOptions{})
	if err != nil {
		t.Fatalf("Prerequisites: %v", err)
	}
	// depth asc, strength desc: beta(0.8) before alpha(0.5)
	if len(prereq) != 2 || prereq[0].Peer.ID != "concept:beta" || prereq[1].Peer.ID != "concept:alpha" {
		t.Fatalf("Prerequisites(gamma)=%v", peerIDs(prereq))
	}
	if prereq[0].RationaleES == "" || prereq[0].Kind != knowledge.EdgeRequires {
		t.Fatalf("Relation should carry edge fields: %+v", prereq[0])
	}

	deps, err := g.Dependents(ctx, knowledge.ConceptID("concept:alpha"), knowledge.TraverseOptions{})
	if err != nil {
		t.Fatalf("Dependents: %v", err)
	}
	if len(deps) != 2 {
		t.Fatalf("Dependents(alpha)=%v", peerIDs(deps))
	}
	if deps[0].Peer.ID != "concept:beta" || deps[1].Peer.ID != "concept:gamma" {
		t.Fatalf("Dependents(alpha) order=%v want [beta, gamma]", peerIDs(deps))
	}

	for _, id := range g.ConceptIDs() {
		p := map[knowledge.ConceptID]struct{}{}
		prereqs, err := g.Prerequisites(ctx, id, knowledge.TraverseOptions{})
		if err != nil {
			t.Fatalf("Prerequisites(%s): %v", id, err)
		}
		for _, r := range prereqs {
			p[r.Peer.ID] = struct{}{}
		}
		dependents, err := g.Dependents(ctx, id, knowledge.TraverseOptions{})
		if err != nil {
			t.Fatalf("Dependents(%s): %v", id, err)
		}
		for _, r := range dependents {
			if _, ok := p[r.Peer.ID]; ok {
				t.Fatalf("direction invariant violated for %s: %s in both Prerequisites and Dependents", id, r.Peer.ID)
			}
		}
	}

	path, err := g.Path(ctx, "concept:gamma", "concept:alpha", knowledge.TraverseOptions{})
	if err != nil {
		t.Fatalf("Path gamma→alpha: %v", err)
	}
	if len(path) < 2 || path[0] != "concept:alpha" || path[len(path)-1] != "concept:gamma" {
		t.Fatalf("Path learning order=%v", path)
	}

	alts, err := g.Neighbors(ctx, "concept:gamma", knowledge.TraverseOptions{
		Kinds: []knowledge.EdgeKind{knowledge.EdgeAlternative},
	})
	if err != nil {
		t.Fatalf("Neighbors: %v", err)
	}
	if len(alts) != 1 || alts[0].Peer.ID != "concept:epsilon" {
		t.Fatalf("Neighbors alternative=%v", peerIDs(alts))
	}
	alts2, err := g.Neighbors(ctx, "concept:epsilon", knowledge.TraverseOptions{
		Kinds: []knowledge.EdgeKind{knowledge.EdgeAlternative},
	})
	if err != nil {
		t.Fatalf("Neighbors reverse: %v", err)
	}
	if len(alts2) != 1 || alts2[0].Peer.ID != "concept:gamma" {
		t.Fatalf("Neighbors alternative reverse=%v", peerIDs(alts2))
	}

	g2, _, err := knowledge.LoadFile(fixturePath(t), knowledge.LoadOptions{})
	if err != nil {
		t.Fatalf("LoadFile2: %v", err)
	}
	got1, _ := g.Prerequisites(ctx, gamma, knowledge.TraverseOptions{})
	got2, _ := g2.Prerequisites(ctx, gamma, knowledge.TraverseOptions{})
	if len(got1) != len(got2) {
		t.Fatalf("determinism len %d vs %d", len(got1), len(got2))
	}
	for i := range got1 {
		if got1[i].Peer.ID != got2[i].Peer.ID || got1[i].Strength != got2[i].Strength {
			t.Fatalf("determinism mismatch at %d", i)
		}
	}

	if err := g.Health(ctx); err != nil {
		t.Fatalf("Health: %v", err)
	}
	st := g.Stats()
	if st.Concepts != 5 || st.Edges != 6 {
		t.Fatalf("Stats concepts=%d edges=%d", st.Concepts, st.Edges)
	}
}

func peerIDs(rels []knowledge.Relation) []knowledge.ConceptID {
	out := make([]knowledge.ConceptID, len(rels))
	for i, r := range rels {
		out[i] = r.Peer.ID
	}
	return out
}
