# ADR-002 — Espacios vectoriales separados: contenido ≠ V_e

**Estado:** Aceptado · **Fecha:** 2026-08-03 · **Autores:** destilado desde Olas 1–2 / OpenSpec

## Contexto

El ruteo pedagógico y el perfil adaptativo del estudiante no miden lo mismo.
Mezclar dimensiones (truncar, pad silencioso o sumar deltas del espacio de
contenido sobre $V_e$) produce preferencias corruptas y matches falsos.

## Decisión

Mantener **dos espacios explícitos**:

1. **Contenido** — embeddings de nodos, queries, chunks RAG y `vector_delta` de
   botones live: `vector.ContentEmbedDims` (**64** en modo hash offline; dims
   remotas vía embedder HTTP).
2. **Preferencia del estudiante ($V_e$)** — cinco ejes
   Dominio / Sensorial / Frustración / Ritmo / Autonomía: `dua.VeDims = 5`.

Prohibido aplicar deltas del espacio de contenido sobre $V_e$. Los mismatches
de dims fallan o se descartan con log — nunca truncado silencioso entre espacios.

## Consecuencias

- Seeds, índice k-NN y RAG comparten contrato de contenido.
- Snapshots de perfil (`ve_dims`) se rechazan si no coinciden con `VeDims`.
- Cualquier feature nueva debe declarar en qué espacio opera.

## Referencias

- Contrato dims: `pkg/vector/dims.go` (`ContentEmbedDims`; comentario «NOT … V_e»).
- $V_e$: `pkg/dua/profile.go` (`VeDims = 5`, `ProfileRepository`).
- Guardrail anti-mezcla: `pkg/dua/preference_lookup.go` («MUST NEVER be applied to V_e»).
- Test dims de mutación live ≠ $V_e$: `pkg/dua/interactive_test.go`
  (`vector_delta dims … want content space … (not V_e)`).
- Upsert 5-dims rechazado: `pkg/vector/dims_test.go` (error debe aclarar
  `not V_e`).
- Sin promover `vector_delta` → $V_e$: `pkg/dua/preference_lookup_test.go`.
- Snapshot: `pkg/dua/profile_file.go` / `profile_file_test.go`
  (`TestFileProfileStoreVeDimsMismatchDiscards`).
- Provenance: `openspec/changes/add-ola2-adaptive-debt/proposal.md`
  (`ContentEmbedDims=64` vs `VeDims=5`); tag Ola 1 `v0.2.0-ola1`.
