# ADR-003 — Backends opcionales con fallback explícito (nunca silencioso)

**Estado:** Aceptado · **Fecha:** 2026-08-03 · **Autores:** destilado del patrón ×6 del repo

## Contexto

Varias dependencias externas (embedder, LLM, STT, store, grafo) deben poder
apagarse en lab/CI sin mentir sobre el estado del sistema. Un fallback
silencioso a hash/extractivo ante URL mal configurada enmascara errores de
despliegue.

## Decisión

Cada backend opcional sigue el mismo contrato:

- **URL/path vacío** → implementación offline/local documentada + log claro.
- **URL/path presente pero inválido** → **error** (no degradar en silencio).
- El consumidor programa contra una **interfaz**; el `*FromEnv` resuelve.

Cinco instancias vigentes + sexta documentada (ADR-001 §4):

| # | Interfaz | Env | Off / fallback documentado |
|---|----------|-----|----------------------------|
| 1 | `rag.Embedder` | `AVLP_EMBEDDING_URL` | `HashEmbedder`; misconfig → error |
| 2 | `rogerian.Synthesizer` | `AVLP_LLM_URL` | extractivo; router loguea disable |
| 3 | `stt.Transcriber` | `AVLP_STT_URL` | cascada Web Speech / sin mic |
| 4 | `dua.ProfileRepository` | `AVLP_PROFILE_STORE_PATH` | memoria si vacío |
| 5 | `knowledge.KnowledgeGraph` | `AVLP_NEO4J_URI` | `NewFromEnv` → `(nil, nil)` + MemoryGraph |
| 6 | ANN (futuro) | `AVLP_ANN_URL` | motor externo; **aún no implementado** |

Hermano del mismo contrato (Ola 7): visitas de concepto vía
`AVLP_CONCEPT_STORE_PATH` vacío → memoria + log (`openConceptVisitStore`).

## Consecuencias

- CI y demos locales corren sin servicios externos.
- Operadores ven en el log qué camino está activo.
- Cuando llegue ANN, no reinventar el contrato: copy-paste del patrón ×1–5.

## Referencias

- Embedder no-silencioso: `pkg/rag/embedder.go` (`DefaultEmbedderE` —
  «no silent fallback to hash»); URL vacía `(nil, nil)` en
  `pkg/rag/embedder_http.go`.
- LLM off: `cmd/router/main.go` («extractive fallback active…»);
  `pkg/rogerian/synthesizer_http.go` (`NewHTTPSynthesizerFromEnv`).
- STT: `pkg/stt/transcriber_http.go`; log UI en `cmd/master-web/main.go`.
- Profile path: `cmd/router/main.go` + `pkg/dua/profile_file.go`.
- Neo4j: `pkg/knowledge/neo4jgraph/graph.go` / `config.go`
  (`NewFromEnv` → `(nil, nil)`); `TestNewFromEnvEmptyURI`.
- Concept visits (hermano): `cmd/router/main.go` (`openConceptVisitStore`);
  `pkg/knowledge/visits_file.go`.
- Sexta (ANN): `docs/adr/adr-001-criterio-lenguajes.md` §4.
