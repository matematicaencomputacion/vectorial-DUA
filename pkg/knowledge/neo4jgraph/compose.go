package neo4jgraph

import (
	"context"
	"errors"

	"github.com/vectorial-dua/avlp/pkg/knowledge"
)

// ReadThrough prefers Primary for curriculum reads; on transport / breaker
// failure it serves Fallback (typically the file MemoryGraph). Routing never
// calls this graph — only Advisor / orientation RPCs.
type ReadThrough struct {
	Primary  knowledge.KnowledgeGraph
	Fallback knowledge.KnowledgeGraph
}

// Compose returns Fallback when Primary is nil; otherwise a ReadThrough pair.
func Compose(primary *Graph, fallback knowledge.KnowledgeGraph) knowledge.KnowledgeGraph {
	if primary == nil {
		return fallback
	}
	return &ReadThrough{Primary: primary, Fallback: fallback}
}

func (r *ReadThrough) Concept(ctx context.Context, id knowledge.ConceptID) (knowledge.Concept, error) {
	if r.Primary != nil {
		c, err := r.Primary.Concept(ctx, id)
		if err == nil || errors.Is(err, knowledge.ErrConceptNotFound) {
			return c, err
		}
	}
	if r.Fallback == nil {
		return knowledge.Concept{}, errOr(errPrimary(r), knowledge.ErrConceptNotFound)
	}
	return r.Fallback.Concept(ctx, id)
}

func (r *ReadThrough) Prerequisites(ctx context.Context, id knowledge.ConceptID, opts knowledge.TraverseOptions) ([]knowledge.Relation, error) {
	return r.rels(ctx, opts, func(g knowledge.KnowledgeGraph) ([]knowledge.Relation, error) {
		return g.Prerequisites(ctx, id, opts)
	})
}

func (r *ReadThrough) Dependents(ctx context.Context, id knowledge.ConceptID, opts knowledge.TraverseOptions) ([]knowledge.Relation, error) {
	return r.rels(ctx, opts, func(g knowledge.KnowledgeGraph) ([]knowledge.Relation, error) {
		return g.Dependents(ctx, id, opts)
	})
}

func (r *ReadThrough) Neighbors(ctx context.Context, id knowledge.ConceptID, opts knowledge.TraverseOptions) ([]knowledge.Relation, error) {
	return r.rels(ctx, opts, func(g knowledge.KnowledgeGraph) ([]knowledge.Relation, error) {
		return g.Neighbors(ctx, id, opts)
	})
}

func (r *ReadThrough) Path(ctx context.Context, from, to knowledge.ConceptID, opts knowledge.TraverseOptions) ([]knowledge.ConceptID, error) {
	if r.Primary != nil {
		p, err := r.Primary.Path(ctx, from, to, opts)
		if err == nil || errors.Is(err, knowledge.ErrNoPath) || errors.Is(err, knowledge.ErrConceptNotFound) {
			return p, err
		}
	}
	if r.Fallback == nil {
		return nil, errOr(errPrimary(r), knowledge.ErrNoPath)
	}
	return r.Fallback.Path(ctx, from, to, opts)
}

func (r *ReadThrough) Health(ctx context.Context) error {
	if r.Primary != nil {
		if err := r.Primary.Health(ctx); err == nil {
			return nil
		}
	}
	if r.Fallback != nil {
		return r.Fallback.Health(ctx)
	}
	return errors.New("neo4jgraph: no graph backend")
}

func (r *ReadThrough) rels(
	ctx context.Context,
	_ knowledge.TraverseOptions,
	call func(knowledge.KnowledgeGraph) ([]knowledge.Relation, error),
) ([]knowledge.Relation, error) {
	if r.Primary != nil {
		rels, err := call(r.Primary)
		if err == nil || errors.Is(err, knowledge.ErrConceptNotFound) {
			return rels, err
		}
	}
	if r.Fallback == nil {
		return nil, errOr(errPrimary(r), knowledge.ErrConceptNotFound)
	}
	return call(r.Fallback)
}

func errPrimary(r *ReadThrough) error {
	if r != nil && r.Primary != nil {
		return errors.New("neo4jgraph: primary failed")
	}
	return errors.New("neo4jgraph: primary missing")
}

func errOr(a, b error) error {
	if a != nil {
		return a
	}
	return b
}
