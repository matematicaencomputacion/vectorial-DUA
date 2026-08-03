# Harness — Control plane AVLP

Suites:

| Suite | Package | Propósito |
|-------|---------|-----------|
| Evals | `harness/evals` | Calidad pedagógica DUA/Rogers del ruteo |
| Simmatrix | `harness/evals` | Matrices query×nodo y query×chunk para calibrar ambos pisos |
| Calibrate | `harness/evals` | Sugiere el umbral estático y opcionalmente lo aplica |
| Bench | `harness/bench` | Latencia `Nearest`/`Retrieve` vs umbrales ADR-001 §4 |
| Sandbox | `harness/sandbox` | Ejecución aislada de celdas |
| Load | `harness/load` | Concurrencia gRPC + percentiles |
| Telemetry | `harness/telemetry` | Contadores, p99, spans LLM |

## CLI

```powershell
go run ./cmd/harness -suite evals
go run ./cmd/harness -suite simmatrix -embedder hash
go run ./cmd/harness -suite calibrate -embedder env --apply
go run ./cmd/harness -suite bench                    # ladder completa 100…100K × 64/1024
go run ./cmd/harness -suite bench -bench-sizes 100,1000   # CI / guard algorítmico
go run ./cmd/harness -suite sandbox
go run ./cmd/harness -suite load -addr 127.0.0.1:50051 -n 500 -c 32
go run ./cmd/harness -suite all
```

Reportes en `harness/out/`.

### Bench (ADR-001 §4)

Mide `Index.Nearest` y `rag.Store.Retrieve` con `testing.Benchmark` sobre índices
sintéticos deterministas (seed fija `20260803`; embeddings L2-normalizados). Dims:
**64** (hash offline) y **1024** (stand-in bge-m3).

- **CI** corre solo `100` y `1_000` nodos/chunks (`-bench-sizes 100,1000`) y
  **falla** si algún escenario de esa escala supera **5 ms/consulta**
  (regresión algorítmica, no de escala). El paso CI usa `go run` **sin**
  `-race` (el detector infla ns/op y no es comparable al umbral ADR-001).
- **Manual (escala):** sin `-bench-sizes` corre también `10_000` y `100_000`.
  Un cruce ahí es señal de reevaluación ANN (ADR-001), no rompe el binario
  salvo que también fallen los escenarios chicos. Volcado:
  tabla stdout + `harness/out/bench.json`.

Ejemplo de línea:

```text
Nearest@10K×1024: 2.1ms — 42% del umbral ADR-001
```

`simmatrix` escribe `simmatrix.json` (query×nodo) y `simmatrix_rag.json`
(query×chunk, con sugerencia para `AVLP_RAG_MIN_SIMILARITY`). `calibrate
--apply` persiste el umbral estático en `data/avlp.json`; `-config` permite
elegir otra ruta.

Faithfulness expone dos modos en `harness/evals`: `extractive` exige que los
chunks aparezcan en la respuesta; `generative` usa precisión de términos
grounded, cobertura de términos clave y atribución de fuente. El segundo es un
guardrail léxico offline: no detecta contradicciones y puede penalizar
paráfrasis válidas, por lo que no reemplaza revisión humana.

## OpenSpec

Change: `openspec/changes/init-harness-and-vector-router/`
