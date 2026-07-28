# Proposal: Deuda técnica Ola 2 — adaptación semántica y persistencia

## Contexto

Ola 1 cerró el bucle adaptativo de $V_e$, el ruteo DUA dinámico desde `StudentState`, el contrato `ContentEmbedDims=64` y los evals/seeds de Compromiso (tag `v0.2.0-ola1`). Esta change **no implementa** nada: registra deuda acordada para la siguiente ola.

## Objetivo

Planificar y priorizar mejoras que desbloqueen ruteo semántico real, persistencia de perfil y limpieza operativa del stack DUA/router.

## Alcance incluido (Ola 2)

1. **`HTTPEmbedder` real** — ✅ PR 2.1 (`feat/ola2-pr-2.1`): cliente OpenAI-compatible, dims del índice vía embedder activo, sin fallback silencioso.
2. **Golden adicional con fraseo natural** — ✅ PR 2.2 / 2.3: paráfrasis + `expected_outcome_hash`; umbral sugerido bge-m3 ~0.55; discriminación entre nodos del mismo tema vía descriptores diferenciados.
3. **Persistencia de `ProfileStore`** — Hoy in-memory; sobrevive solo al proceso del router.
4. **Logger inyectado en `pkg/dua`** — Reemplazar `log.Printf` disperso por interfaz inyectable (tests, niveles, correlación).
5. **Botones legacy (`Botonera` sin schema)** — Fuera del RPC `RecordBotoneraInteraction`; migración o shim explícito.
6. **`go` directive en `go.mod`** — Bajar `go 1.26.5` a la versión mínima real soportada por el código y CI.
7. **Calibración automática de umbral/descriptores** — Parcialmente adelantada por `-suite simmatrix` (matriz query×nodo). Queda automatizar sugerencias de umbral/descriptores al crecer el corpus.

## Fuera de alcance

- UI Master / IDE Antigravity.
- Vector DB externa o HNSW productivo.
- Cambios al contrato `VeDims=5` vs `ContentEmbedDims=64`.

## Riesgos

- Dependencia de servicio de embeddings externo (latencia, costo, disponibilidad).
- Goldens con fraseo natural pueden fallar en CI hasta tener embedder estable.

## Plan de rollback

- Feature flags / build tags para alternar embedder hash vs HTTP en harness y router.
- `ProfileStore` persistente detrás de interfaz; fallback in-memory si el backend falla.
