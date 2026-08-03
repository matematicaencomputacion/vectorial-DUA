package neo4jgraph_test

import (
	"context"
	"os"
	"testing"

	"github.com/vectorial-dua/avlp/pkg/knowledge/neo4jgraph"
)

// Real Bolt smoke. Intentionally gated by RUN_NEO4J_INTEGRATION (not AVLP_*)
// so CI never enables it by ambient config. Prefer an ephemeral local
// container — see docs/neo4j-gcp.md (this package is read-only; graph-sync
// parity tests write and may prune).
//
//	RUN_NEO4J_INTEGRATION=1 AVLP_NEO4J_URI=bolt://127.0.0.1:17687 \
//	  AVLP_NEO4J_USER=neo4j AVLP_NEO4J_PASSWORD=... \
//	  go test ./pkg/knowledge/neo4jgraph/ -run Integration -count=1 -v
func TestNeo4jIntegrationHealth(t *testing.T) {
	if os.Getenv("RUN_NEO4J_INTEGRATION") != "1" {
		t.Skip("set RUN_NEO4J_INTEGRATION=1 (and AVLP_NEO4J_*) for live Bolt")
	}
	g, err := neo4jgraph.NewFromEnv(t.Logf)
	if err != nil {
		t.Fatal(err)
	}
	if g == nil {
		t.Fatal("expected graph when URI set")
	}
	defer g.Close(context.Background())
	if err := g.Health(context.Background()); err != nil {
		t.Fatal(err)
	}
}
