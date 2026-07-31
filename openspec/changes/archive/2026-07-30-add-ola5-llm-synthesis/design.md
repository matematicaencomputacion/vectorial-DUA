## Context

`PromptBuilder` ya produce un `PromptBundle` con duda, tono, dimensión DUA,
contexto RAG numerado y `FullPrompt`. `livestation.Generator` lo renderiza hoy
copiando chunks; `live_content` ya viaja como Markdown por los RPC existentes.
No se modifica Protobuf ni la superficie gRPC/HTTP.

## Goals / Non-Goals

**Goals:**

- Agregar síntesis local/remota opcional sin convertir un LLM en dependencia de CI.
- Mantener fuentes, fallback y errores bajo control determinista de la aplicación.
- Hacer observable qué modo produjo cada estación.
- Mantener retrieval dentro del objetivo p99 existente; la latencia LLM se mide
  aparte y queda limitada por `AVLP_LLM_TIMEOUT` (30s default).

**Non-Goals:**

- Streaming, conversaciones multi-turn, herramientas o judge LLM.
- Garantizar verdad semántica solo mediante una métrica léxica.
- Cambiar el frontend o los contratos de estación.

## Decisions

### La interfaz vive en `pkg/rogerian`

`Synthesizer` recibe `PromptBundle`, tipo propiedad de `pkg/rogerian`; ubicar
interfaz y cliente allí mantiene juntas la política de prompt y su transporte.
Un nuevo `pkg/synthesis` agregaría dependencia hacia `rogerian` para una única
abstracción y separaría artificialmente el contrato de su consumidor.

La interfaz mínima es:

```go
type Synthesizer interface {
    Synthesize(context.Context, PromptBundle) (string, error)
}
```

### Chat Completions compatible y configuración explícita

`HTTPSynthesizer` envía dos mensajes: un system reforzado que prohíbe usar
conocimiento fuera del contexto y el `FullPrompt` como user. Configuración:
`AVLP_LLM_URL`, `AVLP_LLM_MODEL`, `AVLP_LLM_API_KEY` y `AVLP_LLM_TIMEOUT`.
URL vacía significa synthesizer ausente; el modelo default es
`qwen3:4b-instruct`.

El cliente normaliza una base `/v1` a `/v1/chat/completions`, limita el body de
respuesta, reintenta una vez errores 5xx y nunca reintenta 4xx. Alternativa
descartada: reutilizar `HTTPEmbedder`; request/response, dimensionalidad y
política de retries son contratos distintos.

### Fallback en el límite de generación

`Generator` intenta síntesis solo cuando tiene `Synthesizer`. Ante ausencia o
error llama al renderer extractivo existente y usa un `Logf` inyectable. El
router registra al arrancar si usa LLM o fallback, y cada fallo registra la
causa interna. El estudiante recibe contenido válido, no un error técnico.

La sección `Fuentes` nunca se delega al modelo: se elimina una eventual sección
homónima generada y se agrega una lista construida desde `PromptBundle.Sources`.

### Faithfulness generativo léxico, no substring

El modo generativo calcula:

- **Grounded precision:** proporción de términos significativos de la respuesta
  que aparecen en el contexto.
- **Context coverage:** proporción de términos clave del contexto presentes en
  la respuesta.
- **Source attribution:** presencia de la fuente esperada en la sección final.

Reutiliza `RAGSignals`: `Faithfulness=grounded precision`,
`ContextRelevance=0.5*source + 0.5*coverage`, y el aggregate vigente
`0.6*faithfulness + 0.4*context relevance`.

Se elige este juez porque es determinista, offline y auditable. No detecta
contradicciones, puede penalizar paráfrasis válidas y puede ser engañado por
copiar vocabulario; por eso es guardrail de regresión, no evaluación pedagógica
ni factual definitiva.

### Modelo Ollama recomendado

`qwen3:4b-instruct` es el baseline: multilingüe, 2.5 GB cuantizado y adecuado
para memoria limitada de un Mac mini. `qwen3:8b` (~5.2 GB Q4) es la opción de
mayor calidad cuando hay al menos 16 GB y se acepta más latencia. Se evita una
variante VL porque este PR solo consume texto.

## Risks / Trade-offs

- **El modelo alucina pese al prompt** → grounding reforzado, fuentes
  programáticas, métrica offline y fallback; revisión docente antes de promover.
- **Ollama lento o detenido** → timeout, retry 5xx limitado y fallback extractivo.
- **Retry duplica trabajo costoso** → máximo un retry y solo para 5xx.
- **La métrica léxica no entiende contradicciones** → limitación explícita; no
  habilita promoción automática.
- **Logs podrían incluir secretos** → errores no imprimen API keys ni bodies de
  request; la URL puede mostrarse sin headers.

## Migration Plan

1. Desplegar sin `AVLP_LLM_URL`; comportamiento extractivo y CI no cambian.
2. Instalar/pullar el modelo local y configurar URL/modelo.
3. Verificar una estación live y sus fuentes antes de promoverla.
4. Para rollback, quitar `AVLP_LLM_URL`; no hay migración de datos.
