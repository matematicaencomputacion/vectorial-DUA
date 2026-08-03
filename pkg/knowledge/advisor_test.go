package knowledge_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vectorial-dua/avlp/pkg/knowledge"
)

func TestAdvisorRogerianNoJargon(t *testing.T) {
	ctx := context.Background()
	g, _, err := knowledge.LoadFile(filepath.Join("testdata", "curriculum.json"), knowledge.LoadOptions{})
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	visits := knowledge.NewMemoryConceptVisitStore()
	advisor := &knowledge.Advisor{Graph: g, Visits: visits}

	adv, err := advisor.Advise(ctx, "student-a", "concept:gamma")
	if err != nil {
		t.Fatalf("Advise: %v", err)
	}
	if adv.MessageES == "" || len(adv.Gaps) == 0 {
		t.Fatalf("expected gaps, got %+v", adv)
	}
	if !strings.Contains(adv.MessageES, "Beta") && !strings.Contains(adv.MessageES, "Alpha") {
		t.Fatalf("expected peer titles in message: %q", adv.MessageES)
	}
	forbidden := []string{
		"concept:", "KnowledgeGraph", "Neo4j", "DUA", "rogeriano", "grafo",
		"prerequisite", "TraverseOptions", "MemoryGraph", "rationale_es",
	}
	lower := strings.ToLower(adv.MessageES)
	for _, bad := range forbidden {
		if strings.Contains(lower, strings.ToLower(bad)) {
			t.Fatalf("jargon %q found in %q", bad, adv.MessageES)
		}
	}
}

func TestAdvisorPrivacyIsolation(t *testing.T) {
	ctx := context.Background()
	g, _, err := knowledge.LoadFile(filepath.Join("testdata", "curriculum.json"), knowledge.LoadOptions{})
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	visits := knowledge.NewMemoryConceptVisitStore()
	if err := visits.RecordVisit(ctx, "alice", "concept:alpha"); err != nil {
		t.Fatal(err)
	}
	if err := visits.RecordVisit(ctx, "alice", "concept:beta"); err != nil {
		t.Fatal(err)
	}
	advisor := &knowledge.Advisor{Graph: g, Visits: visits}

	alice, err := advisor.Advise(ctx, "alice", "concept:gamma")
	if err != nil {
		t.Fatalf("alice: %v", err)
	}
	if alice.MessageES != "" || len(alice.Gaps) != 0 {
		t.Fatalf("alice should have no gaps, got %+v", alice)
	}

	bob, err := advisor.Advise(ctx, "bob", "concept:gamma")
	if err != nil {
		t.Fatalf("bob: %v", err)
	}
	if len(bob.Gaps) == 0 || bob.MessageES == "" {
		t.Fatalf("bob must see unmet prerequisites: %+v", bob)
	}
	// Bob must not inherit Alice's completion.
	for _, g := range bob.Gaps {
		if g.Peer.ID != "concept:beta" && g.Peer.ID != "concept:alpha" {
			t.Fatalf("unexpected gap %s", g.Peer.ID)
		}
	}
}

func TestFileConceptVisitStorePersists(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "visits.json")
	store, err := knowledge.NewFileConceptVisitStoreWithDebounce(path, 20*time.Millisecond)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	store.Logf = func(string, ...any) {}
	if err := store.RecordVisit(ctx, "stu", "concept:alpha"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for store.WriteCount() == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	reopened, err := knowledge.NewFileConceptVisitStore(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	ok, err := reopened.HasVisited(ctx, "stu", "concept:alpha")
	if err != nil || !ok {
		t.Fatalf("expected persisted visit, ok=%v err=%v", ok, err)
	}
	okOther, _ := reopened.HasVisited(ctx, "other", "concept:alpha")
	if okOther {
		t.Fatal("privacy: other student must not see visit")
	}
}
