# Backlog / deuda — Ola 7

Ítems abiertos que no caben en el PR activo pero quedan rastreables.

## Pendiente (ADR-001 §4)

- [ ] **`harness -suite bench`** — benchmarks de `Nearest` / `Retrieve` con
  100 / 1.000 / 10.000 / 100.000 nodos sintéticos, corriendo en CI, para que
  los umbrales de disparo del [ADR-001 §4](adr-001-criterio-lenguajes.md)
  (latencia p99, escala del índice, RAM) se midan solos y no se descubran
  en producción. Referencia: ADR-001, instrumentación pendiente (PR chico).
