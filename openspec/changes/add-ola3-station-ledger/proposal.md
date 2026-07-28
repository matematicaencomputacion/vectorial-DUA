# Proposal: StationLedger — camino de retorno para estaciones pendientes (Ola 3.a)

## Problema

Ante miss sin RAG usable, `QueryNearestNode` devuelve `LiveStationPending` con `tracking_ulid` y el flujo termina: no hay registro consultable ni retry.

## Decisión de paquete

`StationLedger` vive en **`pkg/vector`** (no `pkg/station`): el ledger es estado del miss path del `Router`, reutiliza `LiveRequest`/`LiveResult`/`NodeNotFoundEvent` y evita un paquete extra para un solo tipo. El RPC `GetLiveStation` consulta `Router.Ledger`.

## PR 5.1

- Registro `in_progress | ready | failed` por ULID
- Wire en `QueryNearestWithOptions`
- `LookupStation` con lazy retry si generator disponible
- TTL `AVLP_STATION_TTL` (default 24h), purge perezoso
- Tests `-race`

## PR 5.2

- Flag `Retrying` + `TryBeginRetry` (un solo `GenerateLive` ante polls concurrentes)
- RPC `GetLiveStation` + `student_message` rogeriano ES
- Pending sin mensaje técnico EN
- `router-client -mode poll`, README ciclo de vida, tests del handler
