## 1. Contrato y cliente HTTP

- [x] 1.1 Definir `Synthesizer` en `pkg/rogerian` junto a `PromptBundle`
- [x] 1.2 Implementar Chat Completions con configuración `AVLP_LLM_*`, timeout y retry 5xx
- [x] 1.3 Cubrir éxito, auth, timeout, retry, errores y resolución por entorno con `httptest`

## 2. Integración de estaciones

- [x] 2.1 Reforzar el prompt para grounding y atribución controlada por la aplicación
- [x] 2.2 Integrar Synthesizer en `livestation.Generator` y wiring del router
- [x] 2.3 Implementar fallback extractivo explícito/logueado para ausencia y fallo
- [x] 2.4 Agregar `Fuentes` programáticamente sin duplicar una sección generada
- [x] 2.5 Cubrir síntesis, fallback, fuentes y telemetría con tests

## 3. Guardrail de fidelidad

- [x] 3.1 Agregar modo generativo con grounded precision, context coverage y atribución
- [x] 3.2 Cubrir respuestas grounded y no soportadas con fixtures offline
- [x] 3.3 Documentar límites y mantener el modo extractivo vigente

## 4. Configuración y verificación

- [x] 4.1 Actualizar tabla `AVLP_*` y setup Ollama con recomendación de modelo
- [x] 4.2 Validar OpenSpec, gofmt, build, vet y `scripts/test-clean.sh`
- [x] 4.3 Verificar localmente síntesis real con Ollama sin convertirla en requisito CI
- [ ] 4.4 Publicar PR 9.1 y confirmar CI verde
