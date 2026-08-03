package knowledge_test

import (
	"context"
	"errors"
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
	if adv.MessageES == "" || len(adv.Gaps) == 0 || !adv.Available {
		t.Fatalf("expected available gaps, got %+v", adv)
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
	if alice.MessageES != "" || len(alice.Gaps) != 0 || !alice.Available {
		t.Fatalf("alice should have no gaps, got %+v", alice)
	}

	bob, err := advisor.Advise(ctx, "bob", "concept:gamma")
	if err != nil {
		t.Fatalf("bob: %v", err)
	}
	if !bob.Available || len(bob.Gaps) == 0 || bob.MessageES == "" {
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

type fakeGraph struct {
	err error
}

func (f fakeGraph) Concept(context.Context, knowledge.ConceptID) (knowledge.Concept, error) {
	return knowledge.Concept{}, f.err
}
func (f fakeGraph) Prerequisites(context.Context, knowledge.ConceptID, knowledge.TraverseOptions) ([]knowledge.Relation, error) {
	return nil, f.err
}
func (f fakeGraph) Dependents(context.Context, knowledge.ConceptID, knowledge.TraverseOptions) ([]knowledge.Relation, error) {
	return nil, f.err
}
func (f fakeGraph) Neighbors(context.Context, knowledge.ConceptID, knowledge.TraverseOptions) ([]knowledge.Relation, error) {
	return nil, f.err
}
func (f fakeGraph) Path(context.Context, knowledge.ConceptID, knowledge.ConceptID, knowledge.TraverseOptions) ([]knowledge.ConceptID, error) {
	return nil, f.err
}
func (f fakeGraph) Health(context.Context) error { return f.err }

func TestAdvisorTransportDegradesAvailableFalse(t *testing.T) {
	ctx := context.Background()
	var logs int
	advisor := &knowledge.Advisor{
		Graph: fakeGraph{err: errors.New("bolt: dial tcp timeout")},
		Logf: func(string, ...any) { logs++ },
	}
	adv, err := advisor.Advise(ctx, "stu", "concept:gamma")
	if err != nil {
		t.Fatalf("transport must not surface as error: %v", err)
	}
	if adv.Available || adv.MessageES != "" || len(adv.Gaps) != 0 {
		t.Fatalf("want Available:false empty advice, got %+v", adv)
	}
	if logs != 1 {
		t.Fatalf("expected one cooldown log, got %d", logs)
	}
	// Second failure within cooldown: no extra log.
	adv2, err := advisor.Advise(ctx, "stu", "concept:gamma")
	if err != nil || adv2.Available {
		t.Fatalf("second call: err=%v adv=%+v", err, adv2)
	}
	if logs != 1 {
		t.Fatalf("cooldown should suppress duplicate logs, got %d", logs)
	}
}

func TestAdvisorEmptyFocusIsCallerError(t *testing.T) {
	advisor := &knowledge.Advisor{Graph: fakeGraph{}}
	_, err := advisor.Advise(context.Background(), "stu", "")
	if err == nil {
		t.Fatal("expected caller error for empty focus")
	}
}
