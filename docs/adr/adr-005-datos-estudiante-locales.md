# ADR-005 — Datos del estudiante siempre locales

**Estado:** Aceptado · **Fecha:** 2026-08-03 · **Autores:** destilado Ola 5–7

## Contexto

Orientación curricular y matching pueden usar infra compartida (Neo4j, índice),
pero el estado del aprendiz ($V_e$, visitas a conceptos, sesión) no debe filtrarse
entre estudiantes ni salir a backends de currículum.

## Decisión

1. **Perfiles $V_e$** solo en proceso o archivo local
   (`AVLP_PROFILE_STORE_PATH`); nunca enviados a Neo4j/RAG/LLM como PII de grafo.
2. **Visitas a conceptos** por `student_id` en store local
   (`AVLP_CONCEPT_STORE_PATH` o memoria).
3. **Cypher Neo4j** del currículum **no** contiene el token `student` —
   solo conceptos/aristas.
4. El Advisor aisla evidencia: las visitas de A no tapan gaps de B.

## Consecuencias

- Neo4j permanece réplica de currículum read-only, no CRM.
- Tests de privacidad son barrera de CI ante regresiones Cypher.
- Persistencia multi-instancia de perfiles queda fuera de alcance (snapshot local).

## Referencias

- Cypher sin `student`: `pkg/knowledge/neo4jgraph/queries.go` (comentario privacy);
  `TestNeo4jQueriesCarryNoStudentData` en `pkg/knowledge/neo4jgraph/graph_test.go`.
- Aislamiento Advisor: `openspec/specs/concept-advisor/spec.md` (scenario
  Aislamiento); `pkg/knowledge/advisor_test.go` («privacy: other student must
  not see visit»).
- Profile store local: `pkg/dua/profile_file.go`; arranque
  `cmd/router/main.go` (`AVLP_PROFILE_STORE_PATH`).
- Concept visits: `pkg/knowledge/visits_file.go` (`FileConceptVisitStore`);
  arranque `cmd/router/main.go` (`AVLP_CONCEPT_STORE_PATH`).
- Compose Neo4j: `pkg/knowledge/neo4jgraph/compose.go` (sólo orientación;
  routing no depende de Neo4j).
