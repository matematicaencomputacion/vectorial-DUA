## Requirement: Neo4j optional read-through

The system SHALL load curriculum orientation from Neo4j when `AVLP_NEO4J_URI`
is set, with file `MemoryGraph` as failover. Routing SHALL NOT depend on Neo4j.

### Scenario: URI empty

- **WHEN** `AVLP_NEO4J_URI` is empty
- **THEN** `NewFromEnv` returns `(nil, nil)` and the router uses MemoryGraph only

### Scenario: Privacy

- **WHEN** Cypher constants are inspected
- **THEN** none contain the token `student`
