# Spec: Graph Sync

## Purpose

Sincronización idempotente Git→Neo4j del currículum. La fuente de verdad es
git; Neo4j es réplica de lectura.

## Requirements

### Requirement: Git is source of truth

Sync SHALL validate curriculum with LoadFile before any Neo4j write.
Manual Neo4j edits are not authoritative.

#### Scenario: dry-run

- **WHEN** `-dry-run` is set
- **THEN** no Bolt write occurs
