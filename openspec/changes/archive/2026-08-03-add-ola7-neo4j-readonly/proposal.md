## Why

El currículum en archivo alcanza para CI; producción necesita Neo4j read-only
sin acoplar el ruteo ni la evidencia de estudiante al grafo remoto.

## What Changes

- `pkg/knowledge/neo4jgraph` + driver `neo4j-go-driver/v5`
- NewFromEnv (nil,nil) con URI vacía; breaker 3 + cooldown; caché TTL
- Read-through MemoryGraph archivo como respaldo
- Privacidad: Cypher sin token `student`; CI sin Neo4j

## Out of Scope

Writes Neo4j, sync curriculum→Bolt, cambios de routing k-NN.
