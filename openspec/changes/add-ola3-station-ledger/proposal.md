# Proposal: StationLedger — camino de retorno para estaciones pendientes (Ola 3.a)

## Estado

**Ola 3.a / arista C2 cerrada** (PR 5.1 + PR 5.2 ✅). Tag: `v0.3.0-ola3a`. Merge en `main`: `feat/ola3-pr-5.2` (`--no-ff`).

## Problema

Ante miss sin RAG usable, `QueryNearestNode` devolvía `LiveStationPending` con `tracking_ulid` y el flujo terminaba: no había registro consultable ni retry. El estudiante quedaba sin camino de retorno en el momento de mayor frustración.

## Decisión de paquete

`StationLedger` vive en **`pkg/vector`** (no `pkg/station`): el ledger es estado del miss path del `Router`, reutiliza `LiveRequest`/`LiveResult`/`NodeNotFoundEvent` y evita un paquete extra para un solo tipo. El RPC `GetLiveStation` consulta `Router.Ledger`.

## PR 5.1 — ✅

- Registro `in_progress | ready | failed` por ULID
- Wire en `QueryNearestWithOptions`
- `LookupStation` con lazy retry si generator disponible
- TTL `AVLP_STATION_TTL` (default 24h), purge perezoso
- Tests `-race`

## PR 5.2 — ✅

- Flag `Retrying` + `TryBeginRetry` (un solo `GenerateLive` ante polls concurrentes)
- RPC `GetLiveStation` + `student_message` rogeriano ES
- Pending sin mensaje técnico EN
- `router-client -mode poll`, README ciclo de vida, tests del handler
- ULID ausente / `student_id` incorrecto → mismo `NotFound` (sin filtrar existencia)

## Fuera de alcance (olas siguientes)

- Persistencia del ledger fuera de proceso
- UI Master para poll automático
- Cola asíncrona de generación (sigue sync + lazy retry en poll)
