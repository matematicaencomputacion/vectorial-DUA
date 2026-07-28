# Proposal: Persistencia de ProfileStore (Ola 2.c — PR 3.1)

## Problema

`$V_e$` vive en un mapa in-memory y se pierde en cada reinicio del router.

## Diseño

- `ProfileRepository` con contrato mínimo `Get` / `Apply` (consumidores: InteractionStore, router).
- `ProfileStore.Snapshot` / `ReplaceAll` quedan fuera de la interfaz: solo el backend de persistencia los usa para flush/load (evita ensanchar el contrato de consumidores).
- `FileProfileStore`: snapshot JSON versionado, carga tolerante, write atómico (temp+rename), flush debounced (default 1s), `Close()` al shutdown.
- Activación: `AVLP_PROFILE_STORE_PATH`; vacío → in-memory (CI sin disco).

## Fuera de alcance

SQLite / Postgres / multi-instancia.
