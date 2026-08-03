package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/vectorial-dua/avlp/pkg/knowledge"
)

// recordingRunner captures isolated schema vs write executions for unit tests.
type recordingRunner struct {
	calls []runnerCall
}

type runnerCall struct {
	Kind  string // "schema" | "write"
	Stmts []string
}

func (r *recordingRunner) ExecSchema(_ context.Context, cypher string, _ map[string]any) error {
	r.calls = append(r.calls, runnerCall{Kind: "schema", Stmts: []string{cypher}})
	return nil
}

func (r *recordingRunner) ExecWrite(_ context.Context, fn func(run cypherRun) error) error {
	var stmts []string
	run := func(cypher string, _ map[string]any) error {
		stmts = append(stmts, cypher)
		return nil
	}
	if err := fn(run); err != nil {
		return err
	}
	r.calls = append(r.calls, runnerCall{Kind: "write", Stmts: stmts})
	return nil
}

func TestApplyPlanSchemaIsolatedBeforeData(t *testing.T) {
	plan := syncPlan{
		Concepts: []knowledge.Concept{
			{ID: "concept:a", Title: "A", Track: knowledge.TrackPython},
			{ID: "concept:b", Title: "B", Track: knowledge.TrackPython},
		},
		Edges: []knowledge.Edge{
			{From: "concept:b", To: "concept:a", Kind: knowledge.EdgeRequires, Strength: 1},
			{From: "concept:a", To: "concept:b", Kind: knowledge.EdgeDeepens, Strength: 0.5},
		},
		Prune: true,
	}
	rec := &recordingRunner{}
	if err := applyPlanOn(context.Background(), rec, plan, time.Unix(0, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(rec.calls) < 2 {
		t.Fatalf("expected schema + writes, got %#v", rec.calls)
	}
	if rec.calls[0].Kind != "schema" {
		t.Fatalf("schema must run first, got %#v", rec.calls)
	}
	if !strings.Contains(rec.calls[0].Stmts[0], "CREATE CONSTRAINT") {
		t.Fatalf("first call must be constraint: %q", rec.calls[0].Stmts[0])
	}

	var sawWrite bool
	for i, c := range rec.calls {
		if c.Kind == "schema" && i > 0 {
			t.Fatalf("schema after data at index %d: %#v", i, rec.calls)
		}
		if c.Kind == "write" {
			sawWrite = true
			for _, stmt := range c.Stmts {
				if strings.Contains(stmt, "CREATE CONSTRAINT") {
					t.Fatalf("constraint mixed into write tx %d: %q", i, stmt)
				}
			}
		}
	}
	if !sawWrite {
		t.Fatal("expected at least one write transaction")
	}

	kinds := make([]string, 0, len(rec.calls))
	for _, c := range rec.calls {
		kinds = append(kinds, c.Kind)
	}
	// schema, concepts, requires, deepens, prune
	wantPrefix := []string{"schema", "write", "write", "write", "write"}
	if len(kinds) != len(wantPrefix) {
		t.Fatalf("call kinds=%v want %v", kinds, wantPrefix)
	}
	for i := range wantPrefix {
		if kinds[i] != wantPrefix[i] {
			t.Fatalf("call kinds=%v want %v", kinds, wantPrefix)
		}
	}
	// Concepts write contains MERGE Concept; no relationship types.
	if !strings.Contains(rec.calls[1].Stmts[0], "MERGE (c:Concept") {
		t.Fatalf("second call should be concepts merge: %q", rec.calls[1].Stmts[0])
	}
	if strings.Contains(rec.calls[1].Stmts[0], "REQUIRES") {
		t.Fatal("concepts tx must not merge edges")
	}
}

func TestApplyPlanSkipsEmptyEdgeKinds(t *testing.T) {
	plan := syncPlan{
		Concepts: []knowledge.Concept{{ID: "concept:a", Title: "A", Track: knowledge.TrackPython}},
		Edges:    nil,
		Prune:    false,
	}
	rec := &recordingRunner{}
	if err := applyPlanOn(context.Background(), rec, plan, time.Unix(0, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(rec.calls) != 2 || rec.calls[0].Kind != "schema" || rec.calls[1].Kind != "write" {
		t.Fatalf("expected schema+concepts only, got %#v", rec.calls)
	}
}
