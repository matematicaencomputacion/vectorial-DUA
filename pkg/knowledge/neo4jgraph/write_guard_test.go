package neo4jgraph_test

import (
	"regexp"
	"testing"

	"github.com/vectorial-dua/avlp/pkg/knowledge/neo4jgraph"
)

// Write verbs that must never appear in neo4jgraph Cypher (router is read-only).
// Writes belong exclusively in cmd/graph-sync.
var writeCypherRE = regexp.MustCompile(`(?i)\b(CREATE|MERGE|DELETE|DETACH|SET|REMOVE|FOREACH)\b`)

func TestNeo4jGraphPackageHasNoWriteCypher(t *testing.T) {
	for i, q := range neo4jgraph.CypherQueries {
		if writeCypherRE.MatchString(q) {
			t.Fatalf("CypherQueries[%d] contains write verb (must live in cmd/graph-sync only):\n%s", i, q)
		}
	}
}
