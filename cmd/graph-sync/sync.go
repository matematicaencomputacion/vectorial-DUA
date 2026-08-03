package main

import (
	"context"
	"fmt"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/vectorial-dua/avlp/pkg/knowledge"
)

// cypherRunner executes Cypher. Schema modifications MUST use ExecSchema
// (isolated auto-commit / own execution). Data writes use ExecWrite — never
// mix CREATE CONSTRAINT with MERGE/DELETE in the same transaction
// (Neo.ClientError.Transaction.ForbiddenDueToTransactionType).
type cypherRunner interface {
	ExecSchema(ctx context.Context, cypher string, params map[string]any) error
	ExecWrite(ctx context.Context, fn func(run cypherRun) error) error
}

type cypherRun func(cypher string, params map[string]any) error

// driverRunner is the production adapter over neo4j.DriverWithContext.
type driverRunner struct {
	driver neo4j.DriverWithContext
}

func (d *driverRunner) ExecSchema(ctx context.Context, cypher string, params map[string]any) error {
	// ExecuteQuery uses its own implicit transaction — never share with data.
	_, err := neo4j.ExecuteQuery(ctx, d.driver, cypher, params, neo4j.EagerResultTransformer)
	return err
}

func (d *driverRunner) ExecWrite(ctx context.Context, fn func(run cypherRun) error) error {
	session := d.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)
	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		run := func(cypher string, params map[string]any) error {
			_, err := tx.Run(ctx, cypher, params)
			return err
		}
		return nil, fn(run)
	})
	return err
}

func applyPlan(ctx context.Context, driver neo4j.DriverWithContext, plan syncPlan, syncedAt time.Time) error {
	return applyPlanOn(ctx, &driverRunner{driver: driver}, plan, syncedAt)
}

func applyPlanOn(ctx context.Context, r cypherRunner, plan syncPlan, syncedAt time.Time) error {
	// 1) Schema alone — BEFORE any data transaction.
	if err := r.ExecSchema(ctx, cypherConstraint, nil); err != nil {
		return fmt.Errorf("constraint: %w", err)
	}

	// 2) Concepts in their own write transaction(s).
	if len(plan.Concepts) > 0 {
		if err := r.ExecWrite(ctx, func(run cypherRun) error {
			return mergeConcepts(ctx, run, plan.Concepts, syncedAt)
		}); err != nil {
			return err
		}
	}

	// 3) One write transaction per relationship kind.
	for _, kind := range []knowledge.EdgeKind{
		knowledge.EdgeRequires, knowledge.EdgeDeepens, knowledge.EdgeContinues, knowledge.EdgeAlternative,
	} {
		var batch []knowledge.Edge
		for _, e := range plan.Edges {
			if e.Kind == kind {
				batch = append(batch, e)
			}
		}
		if len(batch) == 0 {
			continue
		}
		edgeKind := kind
		edgeBatch := batch
		if err := r.ExecWrite(ctx, func(run cypherRun) error {
			return mergeEdges(ctx, run, edgeKind, edgeBatch, syncedAt)
		}); err != nil {
			return err
		}
	}

	// 4) Prune in its own write transaction.
	if plan.Prune {
		if err := r.ExecWrite(ctx, func(run cypherRun) error {
			return pruneAbsent(ctx, run, plan, syncedAt)
		}); err != nil {
			return err
		}
	}
	return nil
}

func mergeConcepts(_ context.Context, run cypherRun, concepts []knowledge.Concept, syncedAt time.Time) error {
	const batchSize = 50
	for i := 0; i < len(concepts); i += batchSize {
		end := i + batchSize
		if end > len(concepts) {
			end = len(concepts)
		}
		rows := make([]map[string]any, 0, end-i)
		for _, c := range concepts[i:end] {
			rows = append(rows, map[string]any{
				"id":      string(c.ID),
				"title":   c.Title,
				"summary": c.Summary,
				"track":   string(c.Track),
				"tags":    c.Tags,
				"source":  c.Source,
			})
		}
		if err := run(cypherMergeConcepts, map[string]any{
			"rows":     rows,
			"syncedAt": syncedAt,
		}); err != nil {
			return fmt.Errorf("merge concepts: %w", err)
		}
	}
	return nil
}

func mergeEdges(_ context.Context, run cypherRun, kind knowledge.EdgeKind, edges []knowledge.Edge, syncedAt time.Time) error {
	stmt, ok := cypherMergeByKind[kind]
	if !ok {
		return fmt.Errorf("unsupported edge kind %s", kind)
	}
	const batchSize = 50
	for i := 0; i < len(edges); i += batchSize {
		end := i + batchSize
		if end > len(edges) {
			end = len(edges)
		}
		rows := make([]map[string]any, 0, end-i)
		for _, e := range edges[i:end] {
			rows = append(rows, map[string]any{
				"from":         string(e.From),
				"to":           string(e.To),
				"strength":     e.Strength,
				"rationale_es": e.RationaleES,
				"source":       e.Source,
			})
		}
		if err := run(stmt, map[string]any{
			"rows":     rows,
			"syncedAt": syncedAt,
		}); err != nil {
			return fmt.Errorf("merge %s: %w", kind, err)
		}
	}
	return nil
}

func pruneAbsent(_ context.Context, run cypherRun, plan syncPlan, syncedAt time.Time) error {
	ids := make([]string, 0, len(plan.Concepts))
	for _, c := range plan.Concepts {
		ids = append(ids, string(c.ID))
	}
	if err := run(cypherPruneOrphanRels, map[string]any{"syncedAt": syncedAt}); err != nil {
		return fmt.Errorf("prune rels: %w", err)
	}
	if err := run(cypherPruneConcepts, map[string]any{"ids": ids}); err != nil {
		return fmt.Errorf("prune concepts: %w", err)
	}
	return nil
}
