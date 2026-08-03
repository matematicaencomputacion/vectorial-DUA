## Context

El Advisor habla al estudiante; el store aísla visitas por `student_id`.

## Decisions

1. Visitas por concepto estable (`concept:<slug>`), no por node ULID.
2. `FileConceptVisitStore` calcado de perfiles: snapshot JSON versionado,
   debounce, carga tolerante a corrupción.
3. En match: solo `RecordVisit` de los conceptos del nodo (local). La orientación
   se consulta después vía RPC dedicado (PR 7.3), nunca dentro de `QueryNearestNode`.
4. Copy: títulos + `RationaleES`; prohibido ids crudos, DUA, grafo, Neo4j, etc.
5. Degradación en el Advisor: fallas de transporte → `Available:false` + `err=nil`
   con log cooldown; errores de llamador (focus vacío) sí retornan `error`.

## Risks

- Textos draft del currículum llegan al estudiante — misma regla de curaduría.
