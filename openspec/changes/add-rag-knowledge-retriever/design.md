# Technical Design: RAG Knowledge Retriever

## Resumen

El RAG es una extensión del espacio vectorial DUA: los chunks documentales viven en el mismo tipo de índice k-NN (coseno) que los nodos pedagógicos. Ante un miss del router, se recupera contexto verificado, se construye una estación con tono Rogers y se registra como nodo live.

## Flujo

```text
QueryNearest
  ├─ sim ≥ 0.85 → NodeResponse (static, is_live=false)
  └─ sim < 0.85 → NodeNotFoundEvent
                    → rag.Retrieve(topK)
                    → rogerian.PromptBuilder
                    → livestation.Synthesize (template)
                    → vector.Index.RegisterNode (live ULID)
                    → NodeResponse (is_live=true, retrieved_sources[])
```

Si RAG está deshabilitado o falla el ingest, se mantiene `LiveStationPending` (rollback seguro).

## Paquetes

| Paquete | Responsabilidad |
|---------|-----------------|
| `pkg/rag` | Chunking, Embedder, Ingest, Retriever |
| `pkg/rogerian` | PromptBundle con contexto citado |
| `pkg/livestation` | Orquestación miss → live node |
| `pkg/vector` | Coseno / Index (sin cambiar matemática) |

## Embeddings

```go
type Embedder interface {
  Embed(ctx context.Context, text string) ([]float32, error)
  Dims() int
}
```

- Default: `HashEmbedder` dim=64, determinista.
- Opcional: `HTTPEmbedder` (`AVLP_EMBEDDING_URL`, `AVLP_EMBEDDING_API_KEY`).

## Contrato gRPC

`NodeResponse` añade:

```protobuf
repeated string retrieved_sources = 6;
string live_content = 7; // markdown de la estación (MVP)
```

En miss exitoso con RAG, la RPC devuelve `matched` (no `pending`), con `is_live_generated=true`.

## Flag

`AVLP_RAG_ENABLED` (default `true` en router cuando existe knowledge base).

## Evals

Señales: `faithfulness`, `context_relevance` (overlap léxico tokenizado entre respuesta y chunks). PASS si aggregate ≥ 0.80.
