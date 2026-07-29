## 1. Contrato y ciclo de vida

- [x] 1.1 Agregar TTL configurable y purge perezoso de nodos live al índice.
- [x] 1.2 Cubrir expiración, supervivencia de curados, configuración y concurrencia con tests.
- [x] 1.3 Agregar `PromoteLiveStation` y campos persistentes del contenido promovido al protobuf; regenerar stubs.

## 2. Promoción persistente

- [x] 2.1 Implementar construcción y escritura atómica/idempotente del seed promovido.
- [x] 2.2 Actualizar índice y registry como nodo curado conservando `node_id` y embedding.
- [x] 2.3 Implementar el handler canónico en `internal/routerserver` y cablearlo desde `cmd/router`.
- [x] 2.4 Cubrir éxito, replay, estado no listo, ausencia y concurrencia con tests.

## 3. Gateway y experiencia

- [x] 3.1 Exponer `POST /api/stations/{tracking_ulid}/promote` y probar el roundtrip HTTP/gRPC.
- [x] 3.2 Renderizar el markdown persistido de un nodo promovido al cargarlo desde Master.
- [x] 3.3 Documentar `AVLP_LIVE_NODE_TTL`, promoción y trust model del prototipo.

## 4. Verificación

- [x] 4.1 Ejecutar generación de proto, formateo, build, vet y `scripts/test-clean.sh`.
- [ ] 4.2 Publicar PR 8.5 y confirmar CI verde.
