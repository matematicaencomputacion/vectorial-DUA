# Backlog / deuda — Ola 7

Único checklist de seguimiento para deuda abierta de Ola 7. No hay change
OpenSpec paralelo: los seis changes de la ola siguen el ciclo proposal/tasks;
los ítems post-cierre se registran aquí. El criterio técnico vive en el ADR
citado; este archivo es la cola de trabajo.

## Resuelto

- [x] **`harness -suite bench`** — benchmarks de `Nearest` / `Retrieve` con
  100 / 1.000 / 10.000 / 100.000 nodos sintéticos; CI corre 100/1K y falla si
  cruzan el umbral ADR-001 (§4). Ver PR [#26](https://github.com/matematicaencomputacion/vectorial-DUA/pull/26)
  (`feat/bench-pr-12.1`), `harness/bench`, `go run ./cmd/harness -suite bench`.
  Criterio:
  [ADR-001 §4](adr/adr-001-criterio-lenguajes.md).

## Pendiente

_(vacío)_
