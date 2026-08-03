package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/vectorial-dua/avlp/pkg/knowledge"
	"github.com/vectorial-dua/avlp/pkg/knowledge/neo4jgraph"
)

// Same fixture (curriculum.json) loaded in MemoryGraph and synced to Neo4j;
// identical query batteries including order.
func TestParityMemoryGraphVsNeo4j(t *testing.T) {
	if os.Getenv("RUN_NEO4J_INTEGRATION") != "1" {
		t.Skip("set RUN_NEO4J_INTEGRATION=1 (and AVLP_NEO4J_*) for live Bolt parity")
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	repo := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	curriculum := filepath.Join(repo, "data", "knowledge", "curriculum.json")

	mem, _, err := knowledge.LoadFile(curriculum, knowledge.LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}

	cfg := neo4jgraph.ConfigFromEnv()
	if cfg.URI == "" {
		t.Fatal("AVLP_NEO4J_URI required")
	}
	auth := neo4j.NoAuth()
	if cfg.User != "" || cfg.Password != "" {
		auth = neo4j.BasicAuth(cfg.User, cfg.Password, "")
	}
	driver, err := neo4j.NewDriverWithContext(cfg.URI, auth, func(c *neo4j.Config) {
		c.MaxTransactionRetryTime = 2 * time.Second
	})
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close(context.Background())

	plan := buildPlan(mem, true)
	syncedAt := time.Now().UTC()
	if err := applyPlan(context.Background(), driver, plan, syncedAt); err != nil {
		t.Fatalf("sync: %v", err)
	}

	neo, err := neo4jgraph.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer neo.Close(context.Background())

	ctx := context.Background()
	ids := mem.ConceptIDs()
	if len(ids) == 0 {
		t.Fatal("empty curriculum")
	}
	for _, id := range ids {
		mc, err := mem.Concept(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		nc, err := neo.Concept(ctx, id)
		if err != nil {
			t.Fatalf("neo concept %s: %v", id, err)
		}
		if mc.ID != nc.ID || mc.Title != nc.Title || mc.Track != nc.Track {
			t.Fatalf("concept mismatch %s: mem=%+v neo=%+v", id, mc, nc)
		}

		mp, err := mem.Prerequisites(ctx, id, knowledge.TraverseOptions{})
		if err != nil {
			t.Fatal(err)
		}
		np, err := neo.Prerequisites(ctx, id, knowledge.TraverseOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if !relationsEqual(mp, np) {
			t.Fatalf("prerequisites order/content mismatch for %s\nmem=%v\nneo=%v", id, summarize(mp), summarize(np))
		}

		md, err := mem.Dependents(ctx, id, knowledge.TraverseOptions{})
		if err != nil {
			t.Fatal(err)
		}
		nd, err := neo.Dependents(ctx, id, knowledge.TraverseOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if !relationsEqual(md, nd) {
			t.Fatalf("dependents mismatch for %s\nmem=%v\nneo=%v", id, summarize(md), summarize(nd))
		}
	}
}

func relationsEqual(a, b []knowledge.Relation) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Peer.ID != b[i].Peer.ID || a[i].Kind != b[i].Kind || a[i].Depth != b[i].Depth {
			return false
		}
		if a[i].Strength != b[i].Strength {
			return false
		}
	}
	return true
}

func summarize(rels []knowledge.Relation) []string {
	out := make([]string, 0, len(rels))
	for _, r := range rels {
		out = append(out, string(r.Kind)+":"+string(r.Peer.ID))
	}
	return out
}
