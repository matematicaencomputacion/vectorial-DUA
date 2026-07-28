# Design: HTTPEmbedder OpenAI-compatible

## Contrato HTTP

```text
POST {base}/embeddings
Authorization: Bearer {AVLP_EMBEDDING_API_KEY}   (si seteada)
Content-Type: application/json

{"model":"<AVLP_EMBEDDING_MODEL>","input":["<text>"]}
```

Respuesta esperada (OpenAI / Voyage / LM Studio / Ollama `/v1/embeddings`):

```json
{"data":[{"embedding":[...],"index":0}]}
```

Normalización de URL: si no termina en `/embeddings`, se concatena (p. ej. `http://localhost:11434/v1` → `…/v1/embeddings`).

## Resolución de embedder

```text
DefaultEmbedder()
  ├─ AVLP_EMBEDDING_URL set → HTTPEmbedder
  └─ else → HashEmbedder(ContentEmbedDims=64)
```

Sin fallback automático HTTP→hash: el operador elige explícitamente desactivando la URL.

## Dimensionalidad

| Fuente | Comportamiento |
|--------|----------------|
| `AVLP_EMBEDDING_DIMS` | Fija `Dims()` antes del primer call |
| Primer `Embed` exitoso | Cachea `len(embedding)`; calls siguientes deben coincidir |
| `ContentEmbedDims` (64) | Default **solo** del modo hash offline |

Índice de ruteo: `NewIndexWithDims(embedder.Dims())` tras `EnsureEmbedderDims` (probe si dims aún desconocidas).

Proyección de vectores: `FitIndexEmbedding(v, index.Dims())` — pad legacy solo si `wantDims == ContentEmbedDims` y `len(v) < wantDims`; rechazo explícito en cualquier otro mismatch.

## Configuración

| Variable | Default |
|----------|---------|
| `AVLP_EMBEDDING_URL` | — (hash offline) |
| `AVLP_EMBEDDING_API_KEY` | — |
| `AVLP_EMBEDDING_MODEL` | `text-embedding-3-small` |
| `AVLP_EMBEDDING_DIMS` | descubrimiento en runtime |
| `AVLP_EMBEDDING_TIMEOUT` | `10s` |

## Resiliencia

- Timeout por request (configurable).
- 1 reintento en error de red o HTTP 5xx.
- 401/4xx: error descriptivo, sin retry.
