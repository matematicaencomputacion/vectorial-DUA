# Backlog / deuda — Ola 7

Único checklist de seguimiento para deuda abierta de Ola 7. No hay change
OpenSpec paralelo: los seis changes de la ola siguen el ciclo proposal/tasks;
los ítems post-cierre se registran aquí. El criterio técnico vive en el ADR
citado; este archivo es la cola de trabajo.

## Pendiente

- [ ] **`harness -suite bench`** — benchmarks de `Nearest` / `Retrieve` con
  100 / 1.000 / 10.000 / 100.000 nodos sintéticos, corriendo en CI, para que
  los umbrales de disparo del [ADR-001 §4](adr-001-criterio-lenguajes.md)
  (latencia p99, escala del índice, RAM) se midan solos y no se descubran
  en producción. Referencia: ADR-001 §4 (instrumento pendiente, PR chico).
