## Context

El Advisor habla al estudiante; el store aísla visitas por `student_id`.

## Decisions

1. Visitas por concepto estable (`concept:<slug>`), no por node ULID.
2. `FileConceptVisitStore` calcado de perfiles: snapshot JSON versionado,
   debounce, carga tolerante a corrupción.
3. En match: `Advise` con historial previo, luego `RecordVisit` de los conceptos
   del nodo (así el consejo no se autocancela).
4. Copy: títulos + `RationaleES`; prohibido ids crudos, DUA, grafo, Neo4j, etc.

## Risks

- Textos draft del currículum llegan al estudiante — misma regla de curaduría.
