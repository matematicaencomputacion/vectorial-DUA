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

Instancias vigentes (×6):

| # | Interfaz | Env | Off / fallback documentado |
|---|----------|-----|----------------------------|
| 1 | `rag.Embedder` | `AVLP_EMBEDDING_URL` | `HashEmbedder`; misconfig → error |
| 2 | `rogerian.Synthesizer` | `AVLP_LLM_URL` | extractivo; router loguea disable |
| 3 | `stt.Transcriber` | `AVLP_STT_URL` | cascada Web Speech / sin mic |
| 4 | `dua.ProfileRepository` | `AVLP_PROFILE_STORE_PATH` | memoria si vacío |
| 5 | `knowledge.KnowledgeGraph` | `AVLP_NEO4J_URI` | `NewFromEnv` → `(nil, nil)` + MemoryGraph |
| 6 | visitas de concepto | `AVLP_CONCEPT_STORE_PATH` | memoria si vacío |

## Consecuencias

- CI y demos locales corren sin servicios externos.
- Operadores ven en el log qué camino está activo.
- ANN futuro (`AVLP_ANN_URL`) debe repetir este patrón (ADR-001 §4).

## Referencias

- Embedder no-silencioso: `pkg/rag/embedder.go` (`DefaultEmbedderE` —
  «no silent fallback to hash»).
- LLM off: `cmd/router/main.go` («extractive fallback active…»).
- STT FromEnv: `pkg/stt/transcriber_http.go` (`NewHTTPTranscriberFromEnv`).
- Profile path: `cmd/router/main.go` + `pkg/dua/profile_file.go`.
- Neo4j: `pkg/knowledge/neo4jgraph/graph.go` (`NewFromEnv` → `(nil, nil)`);
  test `TestNewFromEnvEmptyURI` en `graph_test.go`.
- Concept visits: `cmd/router/main.go` (`openConceptVisitStore` /
  `AVLP_CONCEPT_STORE_PATH`).
- Synthesizer FromEnv: `pkg/rogerian/synthesizer_http.go`
  (`NewHTTPSynthesizerFromEnv`).
