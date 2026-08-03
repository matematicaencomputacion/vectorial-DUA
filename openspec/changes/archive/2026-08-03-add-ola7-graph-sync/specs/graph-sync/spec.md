## Requirement: Git is source of truth

Sync SHALL validate curriculum with LoadFile before any Neo4j write.
Manual Neo4j edits are not authoritative.

### Scenario: dry-run

- **WHEN** `-dry-run` is set
- **THEN** no Bolt write occurs
