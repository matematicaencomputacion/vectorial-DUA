## Context

El índice k-NN conserva hoy nodos curados y live en las mismas estructuras. `StationLedger` ya aplica TTL perezoso, pero el nodo live indexado no expira. Una estación lista contiene el nodo, embedding, markdown y fuentes necesarios para una promoción. Desde PR #7, `internal/routerserver` es la única implementación gRPC.

## Goals / Non-Goals

**Goals:**

- Acotar el crecimiento del índice sin cambiar el margen de preferencia curado/live.
- Promover una estación lista de forma segura, idempotente y durable.
- Mantener un único handler gRPC y reutilizarlo desde tests y gateway.
- Conservar el contenido promovido tras reiniciar el router.

**Non-Goals:**

- Autenticación o roles docentes; conocer el ULID sigue siendo la capacidad del prototipo.
- Partición live por estudiante, calibración del margen u observabilidad nueva.
- Persistencia de estaciones live no promovidas.

## Decisions

### TTL propiedad del índice

`vector.Node` incorpora `CreatedAt`; solo `RegisterLiveNode` lo completa. El valor cero y `IsLiveGenerated=false` quedan fuera del TTL. `Index` captura `AVLP_LIVE_NODE_TTL` al construirse, con default 24h, y purga bajo lock exclusivo antes de `Nearest`, `Upsert`, `Len`, `Nodes` y `HasULID`.

Se elige purge perezoso, análogo al ledger, porque evita goroutines y ciclo de shutdown. La purga elimina coherentemente `nodes`, `ring` y `order`. El costo sigue siendo O(n), igual que el matching actual, y se mantiene bajo el objetivo de 15ms del prototipo.

### Promoción en una dependencia de dominio inyectada al routerserver

Un `dua.LiveStationPromoter` recibe ledger, índice, registry y directorio de seeds. `internal/routerserver.Server` valida el request, invoca una sola implementación y traduce errores a códigos gRPC. `cmd/router` solo resuelve el directorio y cablea dependencias.

Se conserva el `node_id` live: `Index.Upsert` reemplaza la entrada y `IsLiveGenerated=false` la vuelve curada inmediatamente. No se re-embebe; se reutiliza el embedding que produjo la estación.

### Seed interactivo mínimo y durable

El seed se escribe en el directorio configurado por `AVLP_INTERACTIVE_NODES_DIR` (default `data/nodes/interactive`) como `promoted-{tracking_ulid}.json`. Es el único formato persistente de nodos que el router ya carga.

El nodo promovido contiene una botonera mínima `ask_agent`, markdown por defecto, fuentes y `promoted_from_tracking_ulid`. Esos campos se agregan al contrato interactivo para que `GetInteractiveNode` y Master puedan mostrar el material después de un reinicio.

La escritura usa archivo temporal en el mismo directorio, `Sync` y `Rename`. El nombre se deriva únicamente de un ULID validado. Un mutex serializa promociones y el archivo existente se valida para ofrecer replay idempotente.

### Contrato y endpoint

`PromoteLiveStationRequest` lleva `tracking_ulid`. La respuesta incluye `tracking_ulid`, `node_id`, `seed_path`, `created`, `live_content`, `retrieved_sources` y `dimension_dua`.

El gateway expone `POST /api/stations/{tracking_ulid}/promote`, sin body, y usa el cliente gRPC existente.

## Risks / Trade-offs

- **El lock exclusivo del índice reduce concurrencia de lecturas** → el índice ya recorre O(n); se prioriza consistencia y se cubre con `-race`.
- **Fallo después de escribir y antes de indexar** → el replay lee el seed existente y completa índice/registry con `created=false`.
- **Promoción sin autorización** → queda documentada como capacidad por ULID; auth es requisito previo a multiusuario.
- **Ledger expira antes de promover** → el RPC devuelve `NotFound`; una promoción ya persistida no depende del ledger.

## Migration Plan

1. Desplegar proto, TTL y promoción en conjunto.
2. Los nodos existentes no live permanecen inmortales por `CreatedAt` cero.
3. Los nuevos seeds promovidos se cargan por el flujo interactivo existente.
4. Para rollback, retirar el endpoint; los JSON promovidos siguen siendo seeds curados válidos.
