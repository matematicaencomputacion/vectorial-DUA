## Context

See proposal.md — Why. Hoy el miss materializa la estación, la indexa con
`resource_url=live://stations/{trackingULID}` y guarda `Content`/`Sources` en
`StationLedger`. Un segundo `QueryNearest` del mismo embedding gana el k-NN
sobre ese nodo pero el branch de match estático no consulta el ledger, así que
`LiveContent` sale vacío. El Stage cae al fallback de `mediaUrl` y muestra la
URL.

Con LLM, `GenerateLive` puede tardar decenas de segundos en el mismo RPC. Ola
3.a ya expone `GetLiveStation` + polling en master-web; hoy ese camino queda
casi ocioso cuando el miss espera al modelo.

## Goals / Non-Goals

**Goals:**

- Rematch de nodos live con el mismo markdown/fuentes que la generación.
- Miss path que no bloquee la UI más allá de un deadline configurable.
- Reutilizar ledger + poll sin nuevos RPC.

**Non-Goals:**

- Cola durable, workers separados o cancelación cooperativa del LLM mid-flight.
- Hidratar en el frontend leyendo el ledger.
- Cambiar el umbral k-NN ni la política live-vs-curado.

## Decisions

### 1. Hidratar en `pkg/vector.Router`, no en el frontend ni solo en gRPC

Al match ≥ umbral, si `Node.IsLiveGenerated` y `ResourceURL` tiene prefijo
`live://stations/`, extraer el ULID y, si el ledger tiene `ready` con
`Result`, copiar `LiveContent`, `RetrievedSources`, `TrackingULID` e
`IsLiveGenerated=true`.

**Alternativa considerada:** enriquecer solo en `internal/routerserver`. Peor:
harness/evals y otros callers del router seguirían sin contenido. El servidor
ya mapea `outcome.LiveContent` — no hace falta duplicar.

**Si el ledger expiró pero el nodo sigue en el índice:** devolver el match sin
contenido (no inventar). El Stage mostrará la URL; es un borde de TTL
desalineado ya existente, no empeorado.

### 2. Miss path: sync corto + async por deadline

`AVLP_LLM_SYNC_DEADLINE` (Go duration, default `2s`):

- Arrancar `GenerateLive` en goroutine con contexto desacoplado del RPC
  (`context.WithoutCancel` + el timeout del synthesizer/RAG que ya aplica).
- Esperar hasta el deadline (o `ctx` del request).
- Si termina OK → `Matched` + contenido (comportamiento actual extractivo).
- Si el deadline vence → `LiveStationPending`; la goroutine sigue y hace
  `MarkReady` / `MarkFailed`. El poll de Ola 3.a completa la UI.
- `deadline == 0` → pending inmediato (siempre async).
- Valor inválido → default `2s`.

**Por qué no “siempre async”:** el camino extractivo suele ser <100ms; forzar
pending + poll empeora la UX sin LLM. El deadline separa ambos mundos con una
sola palanca.

**Por qué no “solo si hay synthesizer”:** el deadline es agnóstico al backend;
un RAG lento también se beneficia. Evita acoplar el router a `pkg/rogerian`.

**Flight guard:** se sigue usando `TryBeginRetry` antes de lanzar la
goroutine. Mientras `Retrying`, polls concurrentes no duplican generación.

### 3. Tests

- Unit: registrar live node + `MarkReady` en ledger → segundo
  `QueryNearest` con el mismo embedding trae `LiveContent`.
- Unit: stub lento + deadline corto → primer miss pending; `LookupStation`
  tras completar → ready.
- Playwright: tras chip honesto (o duda novel), repetir la misma query y
  afirmar `live_content` no vacío y Stage con markdown (no URL cruda).

## Risks / Trade-offs

- **[Risk]** Goroutine huérfana si el proceso muere mid-LLM → **Mitigation:**
  igual que hoy ante kill; el ledger queda `in_progress` hasta TTL.
- **[Risk]** Cliente cancela durante la ventana sync → **Mitigation:** devolver
  pending (la generación sigue); el poll recupera el resultado.
- **[Risk]** Rematch sin ledger (TTL) → Stage con URL → **Mitigation:**
  documentado; alinear TTL ledger/índice es deuda aparte.
- **[Trade-off]** Default 2s puede devolver pending en máquinas muy lentas
  incluso extractivo → subir `AVLP_LLM_SYNC_DEADLINE` en ops.

## Migration Plan

Desplegar el binario del router. Sin migración de datos. Rollback: binario
anterior o deadline alto. Documentar `AVLP_LLM_SYNC_DEADLINE` en la tabla
`AVLP_*` del README.
