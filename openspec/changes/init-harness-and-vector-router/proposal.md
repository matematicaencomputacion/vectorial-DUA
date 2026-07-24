# Proposal: Harness de Evaluación + Motor de Ruteo Vectorial AVLP

## Problema

El motor de ruteo vectorial DUA ya resuelve similitud coseno y fallback a estaciones en vivo, pero la plataforma carece de un **arnés de control** que:

1. Evalúe la calidad pedagógica de las recomendaciones DUA (no solo latencia).
2. Aísle la ejecución de código de estudiantes (sandbox).
3. Valide SLO de concurrencia gRPC bajo carga realista.
4. Tracee llamadas a LLMs y métricas p99 de punta a punta.

Sin Harness, el SDD no puede cerrar el ciclo `spec → implement → verify` con evidencia reproducible de calidad, seguridad y rendimiento.

## Objetivo

Incorporar el componente `harness/` como suite oficial de Evals, Sandbox, Load y Telemetría, integrado con el router Go existente (`pkg/vector`, `cmd/router`) bajo OpenSpec (`init-harness-and-vector-router`), manteniendo latencia de ruteo $t < 15\text{ms}$ (p99 in-process) y contratos Protobuf extendidos.

## Alcance incluido

- Estructura canónica del monorepo AVLP (`cmd`, `pkg`, `proto`, `openspec`, `harness`, `gen`, `scripts`).
- OpenSpec change `init-harness-and-vector-router` (proposal, design, tasks, delta specs).
- Contratos `harness_eval.proto` + extensión de telemetría ligada a `student_state.proto`.
- Suite Go:
  - `harness/evals` — framework de evals de ruteo pedagógico DUA.
  - `harness/sandbox` — aislamiento de ejecución de celdas (timeout, resource caps).
  - `harness/load` — runner de carga gRPC concurrente con percentiles.
  - `harness/telemetry` — métricas, spans LLM y latencia p99.
- CLI `cmd/harness` para ejecutar suites (`evals`, `load`, `sandbox`, `all`).
- Datasets de evals golden + reporter JSON.

## Fuera de alcance

- UI completa Master / IDE Antigravity / Agente (solo stubs de paquetes y contratos).
- Orquestador LLM productivo de estaciones en vivo (Harness solo traza/evalúa stubs).
- Contenedores Kubernetes/Firecracker productivos (sandbox usa proceso local aislado con límites; diseño preparado para upgrade).
- Persistencia de evals en warehouse (salida local JSON/NDJSON en esta fase).

## Riesgos

- Falsos negativos en evals si el golden set no cubre preferencias sensoriales DUA.
- Escape de sandbox si se permite ejecución sin allowlist de runtime.
- Distorsión de p99 gRPC por cold-start / antivirus en Windows.

## Plan de rollback

- Desactivar CLI harness (`cmd/harness`) sin afectar `cmd/router`.
- Revertir delta OpenSpec y `harness_eval.proto` sin tocar el motor de ruteo ya estabilizado.
- Mantener `init-adaptive-vector-router` como baseline independiente.
