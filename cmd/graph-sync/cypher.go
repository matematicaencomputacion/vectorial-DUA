package main

import "github.com/vectorial-dua/avlp/pkg/knowledge"

// Write Cypher lives ONLY in cmd/graph-sync. pkg/knowledge/neo4jgraph is read-only.

const cypherConstraint = `
CREATE CONSTRAINT concept_id_unique IF NOT EXISTS
FOR (c:Concept) REQUIRE c.id IS UNIQUE`

const cypherMergeConcepts = `
UNWIND $rows AS row
MERGE (c:Concept {id: row.id})
SET c.title = row.title,
    c.summary = row.summary,
    c.track = row.track,
    c.tags = row.tags,
    c.source = row.source,
    c.synced_at = $syncedAt`

// Per-kind MERGE statements. Cypher does not allow MERGE with a dynamic
// relationship type without APOC (e.g. apoc.merge.relationship) — keep one
// statement per EdgeKind instead of parameterizing type(r).
var cypherMergeByKind = map[knowledge.EdgeKind]string{
	knowledge.EdgeRequires: `
UNWIND $rows AS row
MATCH (a:Concept {id: row.from}), (b:Concept {id: row.to})
MERGE (a)-[r:REQUIRES]->(b)
SET r.strength = row.strength,
    r.rationale_es = row.rationale_es,
    r.source = row.source,
    r.synced_at = $syncedAt`,
	knowledge.EdgeDeepens: `
UNWIND $rows AS row
MATCH (a:Concept {id: row.from}), (b:Concept {id: row.to})
MERGE (a)-[r:DEEPENS]->(b)
SET r.strength = row.strength,
    r.rationale_es = row.rationale_es,
    r.source = row.source,
    r.synced_at = $syncedAt`,
	knowledge.EdgeContinues: `
UNWIND $rows AS row
MATCH (a:Concept {id: row.from}), (b:Concept {id: row.to})
MERGE (a)-[r:CONTINUES]->(b)
SET r.strength = row.strength,
    r.rationale_es = row.rationale_es,
    r.source = row.source,
    r.synced_at = $syncedAt`,
	knowledge.EdgeAlternative: `
UNWIND $rows AS row
MATCH (a:Concept {id: row.from}), (b:Concept {id: row.to})
MERGE (a)-[r:ALTERNATIVE]->(b)
SET r.strength = row.strength,
    r.rationale_es = row.rationale_es,
    r.source = row.source,
    r.synced_at = $syncedAt`,
}

// Rel/node prune: drop curriculum edges not touched by this sync run, then
// concepts not present in the file id list.
const cypherPruneOrphanRels = `
MATCH ()-[r:REQUIRES|DEEPENS|CONTINUES|ALTERNATIVE]->()
WHERE r.synced_at IS NULL OR r.synced_at < $syncedAt
DELETE r`

const cypherPruneConcepts = `
MATCH (c:Concept)
WHERE NOT c.id IN $ids
DETACH DELETE c`
