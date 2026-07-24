# Proposal: Motor RAG para Estaciones en Vivo

## Problema

Cuando el router vectorial no encuentra un nodo DUA estático (similitud coseno &lt; 0.85), hoy solo emite `NodeNotFound` y deja la estación en estado `pending`. Sin recuperación de conocimiento verificado, cualquier sintetizador LLM posterior alucinaría contenido sobre el currículo (Henry, PostGIS, Node, etc.).

## Objetivo

Implementar un motor RAG liviano en Go (`pkg/rag`) que indexe `data/knowledge_base/`, recupere top-k chunks por distancia coseno (reutilizando `pkg/vector`) en $t &lt; 15\text{ms}$ in-process, inyecte el contexto en plantillas rogerianas (`pkg/rogerian`) y materialice una Estación en Vivo persistida con ULID (`is_live_generated=true`).

## Alcance incluido

- OpenSpec change `add-rag-knowledge-retriever`.
- `pkg/rag`: chunker, embedder pluggable (HashEmbedder + HTTP opcional), ingest, retriever.
- Extensión de `pkg/rogerian` con `PromptBuilder`.
- Pipeline `pkg/livestation` integrado en el miss path del router/gRPC.
- Knowledge base seed (Henry/PostGIS/Node).
- Evals de faithfulness / context relevance en harness.
- Extensión Protobuf: `retrieved_sources` en `NodeResponse`.

## Fuera de alcance

- Vector DB externa (Qdrant/pgvector).
- LLM cloud obligatorio para síntesis (template-based en esta fase).
- UI Master/IDE/Agente.
- HNSW productivo.

## Riesgos

- HashEmbedder no captura semántica real; suficiente para demos/tests, no para producción pedagógica.
- Cambio de contrato: miss deja de devolver solo `pending` y puede devolver `matched` live en la misma RPC.

## Plan de rollback

- Feature flag `AVLP_RAG_ENABLED=false` restaura el comportamiento pending-only.
- Revertir delta OpenSpec y `pkg/rag` sin tocar el core coseno/ULID.
