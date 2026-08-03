# ADR-010 — Síntesis LLM con grounding y degradación de primera clase

**Estado:** Aceptado · **Fecha:** 2026-08-03 · **Autores:** destilado Ola 5 LLM synthesis

## Contexto

Las estaciones en vivo necesitan prosa estudiante-facing, pero el modelo no
puede inventar fuera del material recuperado. Además, lab/CI deben funcionar
sin `AVLP_LLM_URL` y ante timeouts del modelo.

## Decisión

1. **Grounding primero:** `PromptBuilder` arma un `PromptBundle` solo desde
   chunks RAG (o camino empty-context explícito); el synthesizer HTTP exige
   `FullPrompt` grounded.
2. **Degradación de primera clase:** sin synthesizer, o si `Synthesize` falla,
   `livestation.Generator` usa renderer **extractivo**, loguea el motivo e
   incrementa `livestation_synthesis_fallback_total` — no se oculta el fallo.
3. **Pendiente async** (`AVLP_LLM_SYNC_DEADLINE`): si la generación no entra en
   la ventana sync, `QueryNearestNode` responde `LiveStationPending` con
   mensaje rogeriano (no stack traces).
4. Suites de faithfulness / tone protegen contra leaks de jerga técnica.

## Consecuencias

- Lab sin Ollama/API sigue materializando estaciones extractivas.
- Telemetría distingue síntesis OK vs fallback.
- Empty-context tiene copy propio (sin fingir fuentes).

## Referencias

- Bundle grounded: `pkg/rogerian/prompt.go` (`PromptBundle`, «no invention
  outside context»).
- Synthesizer: `pkg/rogerian/synthesizer.go` + `synthesizer_http.go`.
- Fallback extractivo: `pkg/livestation/generator.go` (logs
  «extractive fallback»); test `pkg/livestation/generator_test.go`
  (`expected extractive fallback`, counter).
- Faithfulness: `harness/evals/faithfulness.go` (`ScoreRAGFaithfulness`).
- Tone / no leak: `pkg/rogerian/tone_live_test.go`, `prompt_test.go`.
- OpenSpec: `openspec/specs/llm-synthesis/spec.md`;
  archive `openspec/changes/archive/2026-07-30-add-ola5-llm-synthesis/`.
- Deadline → pending: `pkg/vector/router.go` (`LLMSyncDeadlineFromEnv`).
