## Why

Sin un grafo de prerrequisitos el ruteo trata cada duda como átomo aislado: no
hay secuenciación curricular ni convergencia visible entre los programas
(inglés, Python, matemática). Ola 7 introduce el grafo dirigido de conceptos
como currículum versionado (Pedagogy-as-Code).

## What Changes

- Paquete `pkg/knowledge`: conceptos con slug estable, aristas tipadas, carga
  validada, `MemoryGraph` en memoria.
- Campo `concepts` en nodos interactivos + `vector.Node`, binding en arranque.
- `data/knowledge/curriculum.json` versionado en git (borrador para curaduría).
- Wiring mínimo en el router (carga + log). Sin RPC, sin UI, sin Neo4j.

### Autoría de textos al estudiante

Los `rationale_es` del JSON son **propuesta de borrador**. Requieren curaduría
de Dario antes de considerarse definitivos: la regla del proyecto es que el
texto que lee el estudiante lo escribe una persona.

### Fuera de alcance (este PR)

RPC, UI Master, Neo4j, evidencia de estudiante, cambios a `Index.Nearest`.

## Capabilities

### New Capabilities

- `knowledge-graph`: Grafo dirigido concepto↔concepto con binding a recursos.

### Modified Capabilities

Ninguna.

## Impact

`pkg/knowledge`, `pkg/dua`, `pkg/vector`, proto, seeds, `cmd/router`, tests.
