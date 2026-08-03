# ADR-007 — Calibración empírica y expectativas duales por modo

**Estado:** Aceptado · **Fecha:** 2026-08-03 · **Autores:** destilado Ola 2 / Ola 4.d / Ola 7 verify

## Contexto

El umbral coseno y el piso RAG no son universales: el embedder hash
(léxico/plumbing) y un denso (p. ej. bge-m3) viven en escalas distintas.
Una sola expectativa «static» rompe CI según el modo.

## Decisión

1. **Calibrar con datos:** suites `simmatrix` y `calibrate` miden peores
   correctos / mejores incorrectos; `--apply` escribe umbral atómico
   (`data/avlp.json` / `AVLP_CONFIG_PATH`).
2. **Expectativas duales** en goldens de ruteo:
   - `expected_outcome` — modo semántico (`embedder=env`).
   - `expected_outcome_hash` — modo hash; el scorer elige según `Mode`.
3. El stack de verify Playwright pinnea el umbral hash calibrado vigente
   (p. ej. **0.482**, calibrate 2026-08-03) y no mezclarlo con
   `data/avlp.json` del operador.

## Consecuencias

- Cambiar corpus o embedder obliga a re-correr `calibrate`/`simmatrix`.
- Los tests unitarios de typo/normalización usan el piso hash, no el
  umbral semántico 0.55.
- Deuda: automatizar elección de umbral al crecer el corpus (comentario en
  `routing_golden.json`).

## Referencias

- Dualidad en scorer: `harness/evals/scorer.go`
  (`ExpectedOutcomeHash`, `EffectiveExpectedOutcome`).
- Goldens: `harness/evals/cases/routing_golden.json`
  (`expected_outcome_hash`, comentarios Compromiso / typo scope).
- Calibrate: `harness/evals/calibrate.go`; apply →
  `pkg/vector/threshold_config.go` (`WriteThresholdConfig`).
- Verify pin: `cmd/master-web/verify/hermetic-stack.mjs`
  (`HASH_CALIBRATED_THRESHOLD = "0.482"`).
- Checklist: `cmd/master-web/MANUAL_CHECKLIST.md` (§Playwright hermético).
- Test typo bajo floor hash: `cmd/router/query_normalization_test.go`
  (`hashCalibratedThreshold = 0.482`).
- OpenSpec: `openspec/changes/archive/2026-07-30-add-ola4-routing-robustness/`.
