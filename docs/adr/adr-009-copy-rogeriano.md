# ADR-009 — Copy rogeriano sin jerga interna

**Estado:** Aceptado · **Fecha:** 2026-08-03 · **Autores:** Dario Bublitz (curaduría), agente + revisión de arquitectura (borrador y verificación)

## Contexto

Orientación, pending live y síntesis hablan al estudiante. Filtrar jerga
(DUA, Neo4j, `concept:`, KnowledgeGraph, «contexto verificado») es requisito
pedagógico, no cosmético.

## Decisión

Todo texto que lee el estudiante lo escribe una **persona**. El LLM **jamás**
escribe `rationale_es` (origen: incidente de andamiaje interno filtrado a un
alumno). La curaduría humana es parte del pipeline, no un extra.

Todo copy estudiante-facing pasa por builders/tono rogeriano:
`Advisor.Advise`, mensajes `LiveStationPending`, `PromptBuilder` / tone live.
Sin candados, sin gamificación: los prerrequisitos **invitan sin bloquear**.
Los tokens baneados (`bloqueado`, `nivel`, `puntos`, `racha`, `%`) son la
versión ejecutable de esa regla, no su definición; los tests de jerga de
plataforma refuerzan el mismo contrato.

## Consecuencias

- Cambios de copy requieren test de leak.
- La UI («Para ubicarte») no educa en jerga de plataforma.
- Límites del enforcement (deuda): el guard de `advisor_test.go` corre sobre
  fixtures, no sobre `data/knowledge/curriculum.json` real;
  `studentFacingRationale` solo remueve el prefijo `[BORRADOR]` — el texto del
  rationale llega tal cual. Pendiente (PR chico): correr los tokens del guard
  sobre el currículum real en el harness.

## Referencias

- `pkg/knowledge/advisor_test.go` (tokens prohibidos / jerga).
- `pkg/knowledge/advisor.go` (`studentFacingRationale`).
- `pkg/rogerian/tone_live_test.go`, `prompt_test.go`.
- `cmd/master-web/web/js/orientation.js` («Para ubicarte»).
- Spec: `openspec/specs/concept-advisor/spec.md`.
