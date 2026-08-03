## Why

El grafo de Ola 7.1 modela currículum, pero sin evidencia local el ruteo no
puede avisar prerequisitos faltantes con tono rogeriano. Ola 7.2 agrega visitas
por estudiante y un Advisor que usa `Relation.RationaleES` ya resuelto.

## What Changes

- `ConceptVisitStore` + `FileConceptVisitStore` (`AVLP_CONCEPT_STORE_PATH`,
  patrón de `FileProfileStore`).
- `Advisor` con copy en español, sin jerga de sistema.
- Wiring: registrar visitas en `QueryNearestNode` (match) y `Record*`.
  La orientación al estudiante va por RPC aparte (PR 7.3), no en la respuesta
  de búsqueda.

### Fuera de alcance

UI Master, Neo4j, RPC nuevo (solo campo en respuesta existente).

## Capabilities

### New Capabilities

- `concept-advisor`: evidencia de visitas + consejo de prerrequisitos.

### Modified Capabilities

- `knowledge-graph`: firmas ya Neo4j-ready; este change las consume.

## Impact

`pkg/knowledge`, `internal/routerserver`, `cmd/router`, proto `NodeResponse`.
