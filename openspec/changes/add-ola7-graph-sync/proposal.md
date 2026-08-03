## Why

Neo4j read-only necesita un camino de escritura operacional controlado desde Git.

## What Changes

- `cmd/graph-sync` (-dry-run, -prune, -validate-seeds; LoadFile antes de Bolt)
- Constraints + MERGE por kind + synced_at; runbook GCP
- Parity test detrás de RUN_NEO4J_INTEGRATION; write-guard en neo4jgraph

## Out of Scope

Escrituras desde el router; edición manual como fuente de verdad.
