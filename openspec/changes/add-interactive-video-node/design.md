# Technical Design: Interactive Video Node DUA

## Concepto UX (contrato Master)

```text
+---------------------------------------------------+---------------------+
|               REPRODUCTOR CENTRAL (STAGE) ~70%    |  BOTONERA DUA ~30%  |
|         Video / diagrama según botón activo       |  clips atomizados   |
|                                                   |  + duda diferente   |
+---------------------------------------------------+---------------------+
|  Barra progreso DUA / Story Points / Siguiente    |
+-------------------------------------------------------------------------+
```

La UI React futura consume `InteractiveVideoNode` y actualiza `activeMedia` sin recargar.

## Modelo

Ver `proto/interactive_node.proto`:

- `LayoutType.INTERACTIVE_DASHBOARD`
- `InteractiveButton` con `PLAY_CLIP` | `OPEN_IDE_CELL` | `ASK_AGENT`
- `is_live_generated` marca botones creados por mutación RAG

## Flujo mutación

```text
MutateInteractiveNode(node_id, doubt_text)
  → registry.Get(node_id)
  → rag.RetrieveText(doubt)
  → rogerian.PromptBuilder (opcional hint)
  → InteractiveButton{ PLAY_CLIP, media_url=live://..., is_live=true, vector_delta }
  → registry.AppendButton
  → return button + nodo actualizado
```

## RPCs (VectorRouter)

- `GetInteractiveNode(NodeIdRequest) → InteractiveVideoNode`
- `MutateInteractiveNode(MutateInteractiveRequest) → MutateInteractiveResponse`

## Go packages

- `pkg/dua` — tipos, Validate, Registry, Mutator
- Seed: `data/nodes/interactive/variables-scope.json`
- Router carga registry al arranque si `AVLP_INTERACTIVE_NODES` ≠ false
