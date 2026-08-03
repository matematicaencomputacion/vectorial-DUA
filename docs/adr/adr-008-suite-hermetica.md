# ADR-008 — Suite hermética

**Estado:** Aceptado · **Fecha:** 2026-08-03 · **Autores:** destilado de incidentes de CI / verify

## Contexto

Varios falsos verdes y falsos rojos vinieron del entorno del operador: exports
`AVLP_*` heredados, `data/avlp.json` local, seeds `promoted-*.json` personales
en el índice, y umbrales/hash mezclados con expectativas semánticas.

## Decisión

La suite (unit + verify Playwright) debe ser **hermética**:

| Regla | Origen del incidente | Ancla actual |
|-------|----------------------|--------------|
| Limpiar `AVLP_*` del shell antes de `go test` | Export `AVLP_SIMILARITY_THRESHOLD=0.55` enmascaró regresión (`7217425` revert goldens; `a2ef354` script) | `scripts/test-clean.sh`; `internal/testenv/testenv.go` |
| `TestMain` / `Isolate` unset process-wide | Mismo leak vía paquete (`df5b970` aislamiento testenv) | `internal/testenv` (`Clear` / `Isolate`); `TestMain` en pkgs de ruteo |
| CI hermético | Paridad local/CI (`c61df52` / `aa56a41`) | workflow CI + `test-clean.sh` |
| Fixtures verify = `git ls-files` interactive | Untracked/promoted local alteraba el Stage | `cmd/master-web/verify/hermetic-stack.mjs` |
| Rechazar `promoted-*` trackeados | Promoted personal entró a git (#20/#21) | aserto en `hermetic-stack.mjs`; dir `promoted-local/` gitignore |
| Pin umbral hash + `AVLP_CONFIG_PATH` aislado | `data/avlp.json` del operador pisaba calibrate | `hermetic-stack.mjs` (`0.482`, config path inexistente) |
| Vaciar embedder/LLM en verify | Llamadas remotas no deterministas | env del stack hermético |

## Consecuencias

- Operadores usan `./scripts/test-clean.sh` (y CI lo equivale).
- Material personal de promote vive fuera del árbol trackeado.
- Cambiar el corpus canónico obliga a re-calibrar el pin hash del verify.

## Referencias

- Guard shell: `scripts/test-clean.sh`.
- Guard tests: `internal/testenv/testenv.go` (comentario del false-green 0.55).
- Stack: `cmd/master-web/verify/hermetic-stack.mjs`.
- Checklist incidente promoted / pin: `cmd/master-web/MANUAL_CHECKLIST.md`.
- README hermeticidad: sección de tests / env en `README.md`.
- Expectativas duales (complemento): ADR-007.
