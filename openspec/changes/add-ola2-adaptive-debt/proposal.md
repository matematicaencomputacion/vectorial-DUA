# Proposal: Deuda técnica Ola 2 — adaptación semántica y persistencia

## Contexto

Ola 1 cerró el bucle adaptativo de $V_e$, el ruteo DUA dinámico desde `StudentState`, el contrato `ContentEmbedDims=64` y los evals/seeds de Compromiso (tag `v0.2.0-ola1`). Esta change **no implementa** nada: registra deuda acordada para la siguiente ola.

## Objetivo

Planificar y priorizar mejoras que desbloqueen ruteo semántico real, persistencia de perfil y limpieza operativa del stack DUA/router.

## Alcance incluido (Ola 2)

1. **`HTTPEmbedder` real** — ✅ PR 2.1 (`feat/ola2-pr-2.1`): cliente OpenAI-compatible, dims del índice vía embedder activo, sin fallback silencioso.
2. **Golden adicional con fraseo natural** — ✅ PR 2.2 / 2.3: paráfrasis + `expected_outcome_hash`; umbral bge-m3 validado **0.55** (matches 0.665–0.765, fuera-manifold ≤0.386, brecha ~0.28).
3. **Persistencia de `ProfileStore`** — ✅ PR 3.1: `ProfileRepository` + `FileProfileStore` (snapshot JSON via `AVLP_PROFILE_STORE_PATH`). Multi-instancia / SQLite queda para cuando se necesite.
4. **Logger inyectado en `pkg/dua`** — ✅ PR 4.1: campo `Logf` opcional (nil = silencioso); router inyecta `log.Printf`.
5. **Botones legacy (`Botonera` sin schema)** — Fuera del RPC `RecordBotoneraInteraction`; migración o shim explícito.
6. **`go` directive en `go.mod`** — ✅ PR 4.1: bajado a `go 1.25.0` (mínimo real; `grpc` v1.82 exige ≥1.25; 1.24 no compila).
7. **Calibración automática de umbral/descriptores** — 🟡 Parcialmente resuelto: existe `-suite simmatrix` (matriz query×nodo). Falta automatizar la elección del umbral; **re-calibrar con simmatrix al crecer el corpus**.

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
