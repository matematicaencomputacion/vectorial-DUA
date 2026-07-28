# Proposal: StationLedger — camino de retorno para estaciones pendientes (Ola 3.a / PR 5.1)

## Problema

Ante miss sin RAG usable, `QueryNearestNode` devuelve `LiveStationPending` con `tracking_ulid` y el flujo termina: no hay registro consultable ni retry.

## Decisión de paquete

`StationLedger` vive en **`pkg/vector`** (no `pkg/station`): el ledger es estado del miss path del `Router`, reutiliza `LiveRequest`/`LiveResult`/`NodeNotFoundEvent` y evita un paquete extra para un solo tipo. El RPC `GetLiveStation` (PR 5.2) consultará `Router.Ledger`.

## Alcance PR 5.1

- Registro `in_progress | ready | failed` por ULID
- Wire en `QueryNearestWithOptions`
- `LookupStation` con lazy retry si generator disponible
- TTL `AVLP_STATION_TTL` (default 24h), purge perezoso
- Tests `-race`

## Fuera de alcance (PR 5.2)

RPC gRPC, mensajes rogerianos, client/README.
