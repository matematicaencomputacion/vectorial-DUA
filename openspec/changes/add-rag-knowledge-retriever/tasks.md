# Tasks: add-rag-knowledge-retriever

## 1. OpenSpec

- [x] Crear proposal, design, tasks y delta `specs/rag/spec.md`.
- [x] Stub `openspec/specs/rag/spec.md`.

## 2. Knowledge base y pkg/rag

- [x] Seed `data/knowledge_base/` (henry, postgis, node).
- [x] Implementar chunker, HashEmbedder, index de chunks, ingest, retriever.
- [x] Tests unitarios de ingest/retrieve.

## 3. Rogerian + Live station

- [x] PromptBuilder con inyección de chunks citados.
- [x] Synthesizer template-based + RegisterNode ULID.
- [x] Integrar en `vector.Router` / `cmd/router` con flag `AVLP_RAG_ENABLED`.

## 4. Protobuf y harness

- [x] Extender `NodeResponse` con `retrieved_sources` y `live_content`.
- [x] Evals faithfulness/relevance + golden RAG.
- [x] Actualizar README y verify.md; marcar checkboxes.
