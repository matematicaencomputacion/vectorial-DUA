## 1. Rematch content

- [x] 1.1 Extraer tracking ULID de `live://stations/{ulid}` e hidratar `LiveContent`/fuentes desde el ledger en el match path de `QueryNearestWithOptions`
- [x] 1.2 Test de regresión: generar/registrar estación ready → segundo QueryNearest con el mismo embedding trae contenido

## 2. Miss path asíncrono

- [x] 2.1 Implementar `AVLP_LLM_SYNC_DEADLINE` (default 2s) con generación en goroutine y pending al vencer
- [x] 2.2 Test: stub lento + deadline corto → pending inmediato; LookupStation tras completar → ready

## 3. Verificación y docs

- [x] 3.1 Playwright: re-preguntar la misma duda live y afirmar Stage con markdown (`live_content`), no URL cruda
- [x] 3.2 Documentar `AVLP_LLM_SYNC_DEADLINE` en README; marcar tasks OpenSpec
