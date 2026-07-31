## Why

Las estaciones live actuales son fieles pero extractivas: ensamblan chunks sin
transformarlos en una explicación adaptada. `PromptBundle` ya contiene contexto
RAG, dimensión DUA y tono rogeriano; falta una síntesis generativa opcional que
lo convierta en contenido pedagógico sin sacrificar el modo local/offline.

## What Changes

- Agrega una interfaz `Synthesizer` junto a `PromptBundle` y una implementación
  HTTP compatible con OpenAI Chat Completions, configurable por `AVLP_LLM_*`.
- Integra síntesis generativa en `livestation.Generator`, con fallback extractivo
  explícito y logueado cuando no hay backend o la llamada falla.
- Refuerza el prompt para usar únicamente contexto citado y agrega la sección
  `Fuentes` programáticamente a toda respuesta generativa.
- Extiende faithfulness con una métrica offline para contenido generativo basada
  en cobertura y precisión de términos clave, documentando sus límites.
- Documenta configuración y setup local con Ollama.

### Incluido

- Cliente HTTP, retries 5xx, timeouts, errores, telemetría y tests `httptest`.
- Ejecución offline/hash sin servicios externos y sin llamadas reales en CI.

### Fuera de alcance

- Cambios de frontend, streaming de tokens, tool calling y persistencia de chats.
- Auth multi-usuario, campos de accesibilidad de medios y voz local.
- Un juez LLM en CI o garantías semánticas equivalentes a evaluación humana.

### Rollback

Eliminar `AVLP_LLM_URL` desactiva inmediatamente la síntesis generativa y
restaura el renderer extractivo. El contrato RPC y los seeds promovidos no
cambian, por lo que no hay migración de datos.

## Capabilities

### New Capabilities

- `llm-synthesis`: Síntesis generativa grounded de estaciones live, fallback
  offline, atribución programática y evaluación de fidelidad generativa.

### Modified Capabilities

Ninguna.

## Impact

Afecta `pkg/rogerian`, `pkg/livestation`, wiring en `cmd/router`, faithfulness
en `harness/evals`, telemetría y documentación. No modifica Protobuf ni la UI.
