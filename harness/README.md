# Harness — Control plane AVLP

Suites:

| Suite | Package | Propósito |
|-------|---------|-----------|
| Evals | `harness/evals` | Calidad pedagógica DUA/Rogers del ruteo |
| Sandbox | `harness/sandbox` | Ejecución aislada de celdas |
| Load | `harness/load` | Concurrencia gRPC + percentiles |
| Telemetry | `harness/telemetry` | Contadores, p99, spans LLM |

## CLI

```powershell
go run ./cmd/harness -suite evals
go run ./cmd/harness -suite sandbox
go run ./cmd/harness -suite load -addr 127.0.0.1:50051 -n 500 -c 32
go run ./cmd/harness -suite all
```

Reportes en `harness/out/`.

## OpenSpec

Change: `openspec/changes/init-harness-and-vector-router/`
