# Technical Design: Harness + Vector Router AVLP

## Resumen

Arquitectura de control en dos planos:

1. **Data plane** — `cmd/router` + `pkg/vector` (ruteo coseno k-NN, ULID, gRPC).
2. **Control plane** — `harness/*` + `cmd/harness` (evals, sandbox, load, telemetry).

OpenSpec gobierna ambos planos; el Harness produce evidencia verificable contra los escenarios Given/When/Then.

## Árbol canónico del monorepo

```text
vectorial-DUA/
├── cmd/
│   ├── router/                 # gRPC VectorRouter :50051
│   ├── router-client/          # smoke client
│   └── harness/                # CLI de suites (evals|load|sandbox|all)
├── pkg/
│   ├── vector/                 # cosine, ULID, k-NN, events, router core
│   ├── dua/                    # (stub) dimensiones DUA / journey helpers
│   └── rogerian/               # (stub) tono de scaffolding / engagement
├── proto/
│   ├── student_state.proto
│   ├── node_schema.proto
│   ├── router_api.proto
│   ├── events.proto
│   └── harness_eval.proto      # NUEVO
├── gen/avlp/vector/v1/         # stubs generados
├── harness/
│   ├── evals/                  # pedagogical routing evals
│   │   ├── cases/              # golden JSON datasets
│   │   ├── scorer.go
│   │   ├── runner.go
│   │   └── report.go
│   ├── sandbox/                # isolated student code execution
│   │   ├── policy.go
│   │   ├── executor.go
│   │   └── result.go
│   ├── load/                   # gRPC concurrency / latency harness
│   │   ├── runner.go
│   │   └── stats.go
│   ├── telemetry/              # metrics + LLM call tracing
│   │   ├── metrics.go
│   │   ├── llm_trace.go
│   │   └── exporter.go
│   └── README.md
├── openspec/
│   ├── config.yaml
│   ├── specs/
│   │   ├── routing/
│   │   └── harness/            # fuente de verdad (post-archive)
│   └── changes/
│       ├── init-adaptive-vector-router/
│       └── init-harness-and-vector-router/
├── scripts/
│   ├── gen-proto.ps1|.sh
│   └── run-harness.ps1
└── README.md
```

## Diagrama de interacción

```text
                 ┌──────────────────────────────────────┐
                 │           cmd/harness (CLI)          │
                 │  evals | sandbox | load | telemetry  │
                 └───────────────┬──────────────────────┘
                                 │
         ┌───────────────────────┼───────────────────────┐
         ▼                       ▼                       ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────────┐
│ harness/evals   │    │ harness/sandbox │    │ harness/load        │
│ DUA quality     │    │ cell isolation  │    │ gRPC fan-out        │
│ golden scoring  │    │ timeout/caps    │    │ p50/p95/p99         │
└────────┬────────┘    └────────┬────────┘    └──────────┬──────────┘
         │                      │                        │
         └──────────┬───────────┴────────────┬───────────┘
                    ▼                        ▼
           ┌────────────────┐      ┌────────────────────┐
           │  pkg/vector    │      │ cmd/router :50051  │
           │  in-process    │◄────►│ VectorRouter gRPC  │
           └────────────────┘      └────────────────────┘
                    │
                    ▼
           ┌────────────────┐
           │harness/telemetry│ → JSON/NDJSON + spans LLM
           └────────────────┘
```

## Harness: Evals de Ruteo Pedagógico

### Objetivo

Medir si el nodo recomendado es pedagógicamente adecuado, no solo cercano en coseno.

### Señales (score 0–1)

| Señal | Peso | Descripción |
|-------|------|-------------|
| `cosine_ok` | 0.35 | Similitud ≥ umbral o fallback live correcto |
| `dua_dimension_match` | 0.30 | Dimensión DUA esperada (Representacion/Accion/Compromiso) |
| `format_match` | 0.20 | Formato (visual/conceptual/practica) alineado a preferencia sensorial |
| `rogers_safety` | 0.15 | No fuerza “deberías saber”; live station si bloqueo básico |

Score agregado ≥ `0.80` ⇒ caso PASS.

### Dataset

`harness/evals/cases/routing_golden.json` — casos con query embedding, expectativa DUA y outcome permitido (`static` | `live`).

## Harness: Sandbox & Execution

- Allowlist de runtimes: `python`, `node` (extensible).
- Timeouts duros, límites de stdout/stderr, working dir temporal.
- Política denegada por defecto (network off en esta fase vía no-redirección; sin shell libre).
- Resultado tipado: `ExitCode`, `Duration`, `Stdout`, `Stderr`, `Violation`.

## Harness: Load & Performance

- Cliente gRPC reutilizando conexión, N workers, M requests.
- Percentiles p50/p95/p99, error rate, QPS.
- SLO: error rate &lt; 1%; p99 in-process router &lt; 15ms; p99 gRPC documentado (ambiente-dependiente).

## Harness: Telemetría

- Contadores: `routing_match_total`, `routing_live_total`, `eval_pass_total`, `sandbox_violation_total`.
- Histogramas de latencia (µs buckets).
- `LLMTraceSpan`: model, prompt_tokens, completion_tokens, latency_ms, purpose (`live_station` | `agent_scaffold`).

## Contratos Protobuf

Ver `proto/harness_eval.proto` y `student_state.proto` (campos de telemetría de sesión opcionales).

## Seguridad

- Sandbox nunca ejecuta el binario del harness como root.
- Evals no mutan el índice de producción; usan índice seed o cliente read-only.
- Secretos LLM vía env (`AVLP_LLM_API_KEY`) — nunca en datasets.

## Rollback técnico

Feature flag CLI: `harness` es opt-in. Router permanece operativo sin harness.
