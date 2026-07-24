# Verification: init-harness-and-vector-router

Fecha: 2026-07-24

## Escenarios Given/When/Then

| Escenario | Resultado | Evidencia |
|-----------|-----------|-----------|
| Eval golden match estático | PASS | `TestGoldenRoutingEvalsPass`; CLI `pass_rate=1.00` (4/4) |
| Eval estación en vivo | PASS | caso `novel-block-live` en golden |
| Sandbox runtime denegado | PASS | `TestSandboxRuntimeDenied` |
| Sandbox timeout | PASS/SKIP | `TestSandboxTimeout` (requiere python) |
| Telemetry snapshot export | PASS | `TestSnapshotExport` + `harness/out/telemetry_snapshot.json` |
| Load gRPC error_rate &lt; 1% | PASS | `cmd/harness -suite load` |

## Comandos

```powershell
go test ./...
go run ./cmd/router
go run ./cmd/harness -suite evals
go run ./cmd/harness -suite sandbox
go run ./cmd/harness -suite load -n 300 -c 24
```
