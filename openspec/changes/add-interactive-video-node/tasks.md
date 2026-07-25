# Tasks: add-interactive-video-node

## 1. OpenSpec

- [x] Crear proposal, design, tasks y delta `specs/interactive_node/spec.md`.
- [x] Stub `openspec/specs/interactive_node/spec.md`.

## 2. Contratos

- [x] Añadir `proto/interactive_node.proto`.
- [x] Extender `router_api.proto` con Get/Mutate RPCs.
- [x] Regenerar stubs Go.

## 3. Dominio Go

- [x] Tipos + Validate en `pkg/dua`.
- [x] Registry concurrente + carga de seed JSON.
- [x] Mutator con RAG.

## 4. Wiring y verificación

- [x] Integrar en `cmd/router`.
- [x] Tests schema + mutación.
- [x] README + verify.md; marcar checkboxes.
