# ADR-004 — Pedagogy-as-Code

**Estado:** Aceptado · **Fecha:** 2026-08-03 · **Autores:** Dario Bublitz (curaduría), agente + revisión de arquitectura (borrador y verificación)

## Contexto

El currículum (conceptos + aristas + seeds interactivos) es material pedagógico
versionado. Editar Neo4j a mano o promover seeds personales al árbol git
rompe la trazabilidad y la hermeticidad del verify.

## Decisión

Git es la **única** fuente de verdad del currículum (`data/knowledge/`,
`data/nodes/interactive/`). Neo4j es réplica de lectura; `graph-sync` valida
con `LoadFile` antes de escribir. Promotes personales viven en
`data/nodes/promoted-local/` (gitignore).

Las aristas viven entre conceptos con slug estable (`concept:<slug>`), **nunca**
entre `node_id`: los ULID de nodos son efímeros (seeds demo se regeneran al
arranque; estaciones live nacen en runtime) y un mismo concepto se enseña con
N recursos DUA. El binding concepto↔recurso se deriva al arrancar y **no** se
sincroniza a la nube.

Neo4j Community no ofrece usuarios read-only (Enterprise). La restricción vive
en el código: el router solo lee (`write-guard` sobre `CypherQueries` en
`pkg/knowledge/neo4jgraph`) y `cmd/graph-sync` es el **único** escritor.

El flujo es **git → Neo4j**, nunca de vuelta. Lo editado a mano en la réplica
lo pisa el próximo sync.

## Consecuencias

- Pedagogy-as-Code: PRs revisan cambios curriculares como código.
- La réplica es desechable: recrear VM + contenedor + rotar credenciales +
  `graph-sync` y el grafo resucita idéntico — verificado **2026-08-03** (sync
  inaugural 20 conceptos / 20 aristas; dos corridas consecutivas con salida
  idéntica = idempotencia MERGE).

## Referencias

- `README.md` § Grafo de conocimiento; `docs/neo4j-gcp.md` (§ verificación /
  idempotencia; advertencia Browser).
- `cmd/graph-sync/`; `openspec/specs/graph-sync/spec.md`.
- Write-guard: `pkg/knowledge/neo4jgraph/write_guard_test.go`
  (`TestNeo4jGraphPackageHasNoWriteCypher`).
- Tag `v0.6.0` mensaje Pedagogy-as-Code.
