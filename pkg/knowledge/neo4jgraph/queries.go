package neo4jgraph

// Cypher templates for curriculum reads. Parameter names are concept ids / thresholds
// only — never student identifiers. Privacy guard: TestNeo4jQueriesCarryNoStudentData.
const (
	cypherHealth = `RETURN 1 AS ok`

	cypherConcept = `
MATCH (c:Concept {id: $id})
RETURN c.id AS id, c.title AS title, c.summary AS summary,
       c.track AS track, c.tags AS tags, c.source AS source`

	// Direct prerequisites (outgoing REQUIRES toward foundations).
	cypherPrerequisites = `
MATCH (from:Concept {id: $id})-[r:REQUIRES]->(to:Concept)
WHERE coalesce(r.strength, 1.0) >= $minStrength
RETURN to.id AS peer_id, to.title AS peer_title, to.summary AS peer_summary,
       to.track AS peer_track, to.tags AS peer_tags, to.source AS peer_source,
       type(r) AS kind, coalesce(r.strength, 1.0) AS strength,
       coalesce(r.rationale_es, '') AS rationale_es, coalesce(r.source, '') AS source,
       1 AS depth
ORDER BY strength DESC, peer_id ASC
LIMIT $limit`

	cypherDependents = `
MATCH (from:Concept)-[r:REQUIRES]->(to:Concept {id: $id})
WHERE coalesce(r.strength, 1.0) >= $minStrength
RETURN from.id AS peer_id, from.title AS peer_title, from.summary AS peer_summary,
       from.track AS peer_track, from.tags AS peer_tags, from.source AS peer_source,
       type(r) AS kind, coalesce(r.strength, 1.0) AS strength,
       coalesce(r.rationale_es, '') AS rationale_es, coalesce(r.source, '') AS source,
       1 AS depth
ORDER BY strength DESC, peer_id ASC
LIMIT $limit`

	cypherNeighbors = `
MATCH (a:Concept {id: $id})-[r]->(b:Concept)
WHERE type(r) IN $kinds AND coalesce(r.strength, 1.0) >= $minStrength
RETURN b.id AS peer_id, b.title AS peer_title, b.summary AS peer_summary,
       b.track AS peer_track, b.tags AS peer_tags, b.source AS peer_source,
       type(r) AS kind, coalesce(r.strength, 1.0) AS strength,
       coalesce(r.rationale_es, '') AS rationale_es, coalesce(r.source, '') AS source,
       1 AS depth
ORDER BY strength DESC, peer_id ASC
LIMIT $limit`

	cypherPath = `
MATCH (src:Concept {id: $from}), (dst:Concept {id: $to})
MATCH p = shortestPath((src)-[:REQUIRES|DEEPENS|CONTINUES*1..$depth]-(dst))
RETURN [n IN nodes(p) | n.id] AS ids`
)

// CypherQueries is the exhaustive list of curriculum Cypher strings used by this package.
// Kept for the privacy invariant test (no "student" token in any query).
var CypherQueries = []string{
	cypherHealth,
	cypherConcept,
	cypherPrerequisites,
	cypherDependents,
	cypherNeighbors,
	cypherPath,
}
