# Tasks: init-harness-and-vector-router

## 1. OpenSpec y estructura

- [x] Crear change `openspec/changes/init-harness-and-vector-router/` (proposal, design, tasks, specs).
- [x] Añadir stub `openspec/specs/harness/spec.md`.
- [x] Documentar árbol canónico AVLP en README.

## 2. Contratos Protobuf

- [x] Añadir `proto/harness_eval.proto` (EvalCase, EvalReport, TelemetrySnapshot, LLMTraceSpan).
- [x] Extender `student_state.proto` con metadatos opcionales de sesión/engagement.
- [x] Regenerar stubs Go (`scripts/gen-proto.ps1`).

## 3. Suite Harness

- [x] Implementar `harness/telemetry` (métricas + LLM spans + export JSON).
- [x] Implementar `harness/evals` (scorer, runner, golden cases, report).
- [x] Implementar `harness/sandbox` (policy + executor con timeout).
- [x] Implementar `harness/load` (gRPC concurrency + percentiles).
- [x] Implementar CLI `cmd/harness`.

## 4. Stubs de dominio futuro

- [x] Crear stubs `pkg/dua` y `pkg/rogerian` (paquetes documentados, sin UI).

## 5. Verificación

- [x] `go test ./...` incluyendo harness.
- [x] Ejecutar `go run ./cmd/harness -suite evals`.
- [x] Ejecutar `go run ./cmd/harness -suite load` contra router local.
- [x] Actualizar checkboxes y `verify.md` del change.
