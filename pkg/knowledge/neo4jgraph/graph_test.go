package neo4jgraph_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/vectorial-dua/avlp/pkg/knowledge"
	"github.com/vectorial-dua/avlp/pkg/knowledge/neo4jgraph"
)

func TestNewFromEnvEmptyURI(t *testing.T) {
	t.Setenv("AVLP_NEO4J_URI", "")
	g, err := neo4jgraph.NewFromEnv(nil)
	if err != nil {
		t.Fatal(err)
	}
	if g != nil {
		t.Fatal("expected nil graph when URI empty")
	}
}

func TestNeo4jQueriesCarryNoStudentData(t *testing.T) {
	for _, q := range neo4jgraph.CypherQueries {
		low := strings.ToLower(q)
		if strings.Contains(low, "student") {
			t.Fatalf("cypher carries student token:\n%s", q)
		}
	}
}

type stubGraph struct {
	conceptFn func(context.Context, knowledge.ConceptID) (knowledge.Concept, error)
	healthErr error
}

func (s *stubGraph) Concept(ctx context.Context, id knowledge.ConceptID) (knowledge.Concept, error) {
	if s.conceptFn != nil {
		return s.conceptFn(ctx, id)
	}
	return knowledge.Concept{}, knowledge.ErrConceptNotFound
}
func (s *stubGraph) Prerequisites(context.Context, knowledge.ConceptID, knowledge.TraverseOptions) ([]knowledge.Relation, error) {
	return nil, nil
}
func (s *stubGraph) Dependents(context.Context, knowledge.ConceptID, knowledge.TraverseOptions) ([]knowledge.Relation, error) {
	return nil, nil
}
func (s *stubGraph) Neighbors(context.Context, knowledge.ConceptID, knowledge.TraverseOptions) ([]knowledge.Relation, error) {
	return nil, nil
}
func (s *stubGraph) Path(context.Context, knowledge.ConceptID, knowledge.ConceptID, knowledge.TraverseOptions) ([]knowledge.ConceptID, error) {
	return nil, knowledge.ErrNoPath
}
func (s *stubGraph) Health(context.Context) error { return s.healthErr }

func TestReadThroughFallsBackOnTransportError(t *testing.T) {
	primary := &stubGraph{
		conceptFn: func(context.Context, knowledge.ConceptID) (knowledge.Concept, error) {
			return knowledge.Concept{}, errors.New("bolt dial timeout")
		},
	}
	fallback := &stubGraph{
		conceptFn: func(_ context.Context, id knowledge.ConceptID) (knowledge.Concept, error) {
			return knowledge.Concept{ID: id, Title: "desde archivo"}, nil
		},
	}
	g := &neo4jgraph.ReadThrough{Primary: primary, Fallback: fallback}
	c, err := g.Concept(context.Background(), "concept:async-await")
	if err != nil {
		t.Fatal(err)
	}
	if c.Title != "desde archivo" {
		t.Fatalf("title=%q", c.Title)
	}
}

func TestReadThroughDoesNotFallbackOnNotFound(t *testing.T) {
	primary := &stubGraph{
		conceptFn: func(context.Context, knowledge.ConceptID) (knowledge.Concept, error) {
			return knowledge.Concept{}, knowledge.ErrConceptNotFound
		},
	}
	fallback := &stubGraph{
		conceptFn: func(context.Context, knowledge.ConceptID) (knowledge.Concept, error) {
			t.Fatal("fallback must not run on ErrConceptNotFound")
			return knowledge.Concept{}, nil
		},
	}
	g := &neo4jgraph.ReadThrough{Primary: primary, Fallback: fallback}
	_, err := g.Concept(context.Background(), "concept:missing")
	if !errors.Is(err, knowledge.ErrConceptNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestComposeNilPrimary(t *testing.T) {
	fb := &stubGraph{}
	g := neo4jgraph.Compose(nil, fb)
	if g != fb {
		t.Fatal("Compose(nil, fb) should return fb")
	}
}
