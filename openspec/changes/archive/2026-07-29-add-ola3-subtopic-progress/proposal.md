# Proposal: Progreso visible de subtemas (Ola 3.c / C4)

## Problema

`InteractionStore.OpenedList` registra desde Ola 1 qué ramas jerárquicas abrió
cada estudiante, pero ese estado no sale del router. El acordeón no puede
recordar “ya visitaste Motor” ni invitar a explorar ramas pendientes.

## Objetivo

Cerrar C4 exponiendo progreso jerárquico como contrato RPC/HTTP y usándolo en
Master con señales accesibles, no gamificadas y de tono rogeriano.

## Alcance

### PR 7.1 — RPC de progreso

- `ProgressForTree(tree, openedSet)` puro en `pkg/dua`.
- Estado por raíz: `visited`, `partial`, `unvisited`.
- Total recursivo y lista de IDs abiertos.
- RPC `GetSubtopicProgress`.
- `GET /api/nodes/{id}/progress?student_id=`.
- Tests unitarios, handler y gateway.

### PR 7.2 — acordeón que recuerda

- Carga y render accesible del progreso.
- Actualización optimista al abrir subtemas.
- Copy rogeriano orientado a autonomía.
- Panel de desarrollo, checklist y evidencia Playwright.

## Restricción de entrega

PRs apilados: `feat/ola3-pr-7.1` y `feat/ola3-pr-7.2`. Cada PR se detiene para
revisión independiente antes de continuar.
