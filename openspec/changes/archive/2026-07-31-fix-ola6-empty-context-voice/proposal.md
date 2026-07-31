## Why

Con RAG vacío, la síntesis LLM filtraba andamiaje interno (DUA, micro-ejercicio,
listas de lo imposible) y dejaba encabezados colgando. El estudiante no debe
ver nunca la plantilla del sistema.

## What Changes

- Rama de prompt vacía en `pkg/rogerian` (2–4 frases + temas curados).
- Poda de secciones vacías y saneamiento de colas `…:` en livestation.
- Tests y asertos Playwright estructurales.

## Impact

`pkg/rogerian`, `pkg/livestation`, `cmd/router`, verify Playwright.
