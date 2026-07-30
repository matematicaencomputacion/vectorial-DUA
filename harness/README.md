# Harness — Control plane AVLP

Suites:

| Suite | Package | Propósito |
|-------|---------|-----------|
| Evals | `harness/evals` | Calidad pedagógica DUA/Rogers del ruteo |
| Simmatrix | `harness/evals` | Matrices query×nodo y query×chunk para calibrar ambos pisos |
| Calibrate | `harness/evals` | Sugiere el umbral estático y opcionalmente lo aplica |
| Sandbox | `harness/sandbox` | Ejecución aislada de celdas |
| Load | `harness/load` | Concurrencia gRPC + percentiles |
| Telemetry | `harness/telemetry` | Contadores, p99, spans LLM |

## CLI

```powershell
go run ./cmd/harness -suite evals
go run ./cmd/harness -suite simmatrix -embedder hash
go run ./cmd/harness -suite calibrate -embedder env --apply
go run ./cmd/harness -suite sandbox
go run ./cmd/harness -suite load -addr 127.0.0.1:50051 -n 500 -c 32
go run ./cmd/harness -suite all
```

Reportes en `harness/out/`.

`simmatrix` escribe `simmatrix.json` (query×nodo) y `simmatrix_rag.json`
(query×chunk, con sugerencia para `AVLP_RAG_MIN_SIMILARITY`). `calibrate
--apply` persiste el umbral estático en `data/avlp.json`; `-config` permite
elegir otra ruta.

## OpenSpec

Change: `openspec/changes/init-harness-and-vector-router/`
