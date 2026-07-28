# Proposal: HTTPEmbedder real (Ola 2 — PR 2.1)

## Problema

El ruteo y el RAG usan `HashEmbedder` por defecto: similitud léxica, no semántica. Con fraseo natural las similitudes caen (~0.14) y el miss path se dispara aunque exista nodo estático relevante. `HTTPEmbedder` existe como stub que devuelve error.

## Objetivo

Implementar cliente HTTP OpenAI-compatible (`POST …/embeddings`) activable por `AVLP_EMBEDDING_URL`, manteniendo `HashEmbedder` como modo offline sin cambios cuando la URL no está configurada.

## Alcance incluido (PR 2.1)

- `pkg/rag/embedder_http.go`: cliente real, timeout (10s default), 1 retry, errores explícitos.
- `DefaultEmbedder()`: URL seteada → HTTP; si no → Hash (64 dims).
- Dimensionalidad: `AVLP_EMBEDDING_DIMS` o descubrimiento en primer call; índice con `NewIndexWithDims(emb.Dims())`; sin truncate/pad silencioso entre dims remotas e índice.
- `EnsureEmbedderDims` para probe al arranque del router.
- Tests unitarios con `httptest` (éxito, 401, dims inconsistentes, timeout).

## Fuera de alcance (PR 2.2+)

- Golden con fraseo natural y runner `embedder: hash|env`.
- Persistencia `ProfileStore`, logger `pkg/dua`, botones legacy, `go.mod` directive.

## Riesgos

- Endpoint caído bloquea arranque del router si URL está seteada (sin fallback silencioso a hash — intencional).
- Dims del embedder remoto distintas de 64 reindexan todo el corpus al activar HTTP.

## Plan de rollback

- Desactivar: unset `AVLP_EMBEDDING_URL` → vuelve HashEmbedder offline.
- Revertir PR 2.1 restaura stub y `NewIndex()` fijo a 64.
