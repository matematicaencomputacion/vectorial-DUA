## Purpose

Evita que las estaciones generadas en vivo crezcan sin límite y permite que una revisión docente convierta material valioso en currículo persistente.

## ADDED Requirements

### Requirement: Los nodos live expiran del índice
El índice SHALL desalojar perezosamente los nodos marcados como live cuando superen `AVLP_LIVE_NODE_TTL`, cuyo valor por defecto SHALL ser 24 horas. Los nodos curados SHALL permanecer sin TTL.

#### Scenario: Live vencida
- **GIVEN** un nodo live cuya antigüedad supera el TTL configurado
- **WHEN** una operación del índice consulta o modifica sus entradas
- **THEN** el nodo live deja de participar del matching y se libera su ULID del índice

#### Scenario: Curado no vence
- **GIVEN** un nodo curado más antiguo que el TTL live
- **WHEN** el índice ejecuta su purga perezosa
- **THEN** el nodo curado permanece disponible

#### Scenario: Configuración inválida
- **GIVEN** un valor vacío, inválido o no positivo de `AVLP_LIVE_NODE_TTL`
- **WHEN** se crea el índice
- **THEN** el sistema usa 24 horas

### Requirement: Una estación lista puede promoverse a curada
El servicio SHALL exponer `PromoteLiveStation(tracking_ulid)` para convertir manualmente una estación live lista en un nodo curado con el mismo `node_id`.

#### Scenario: Primera promoción
- **GIVEN** una estación lista con contenido, fuentes y embedding válidos
- **WHEN** el docente solicita su promoción
- **THEN** el servicio escribe un seed JSON persistente bajo `data/nodes/interactive/`, registra el nodo como curado y devuelve `created=true`

#### Scenario: Repetición idempotente
- **GIVEN** una estación ya promovida
- **WHEN** se repite la promoción con el mismo `tracking_ulid`
- **THEN** el servicio devuelve el mismo `node_id` y seed con `created=false` sin duplicar datos

#### Scenario: Estación ausente o expirada
- **GIVEN** un `tracking_ulid` que no existe en el ledger
- **WHEN** se solicita promoción
- **THEN** el servicio devuelve `NotFound`

#### Scenario: Estación aún no lista
- **GIVEN** una estación en progreso o fallida
- **WHEN** se solicita promoción
- **THEN** el servicio devuelve `FailedPrecondition` y no escribe un seed

### Requirement: El seed promovido conserva el material revisado
El seed persistente SHALL conservar el contenido markdown, las fuentes recuperadas, el embedding y el vínculo al `tracking_ulid` original para que el nodo siga siendo utilizable después de reiniciar el router.

#### Scenario: Recarga desde disco
- **GIVEN** un seed promovido escrito correctamente
- **WHEN** un nuevo proceso carga el directorio de nodos interactivos
- **THEN** el nodo se registra como curado y expone el mismo contenido y fuentes

### Requirement: La promoción está disponible por HTTP
El gateway SHALL exponer `POST /api/stations/{tracking_ulid}/promote` y mapear su respuesta y errores al RPC canónico.

#### Scenario: Promoción vía gateway
- **GIVEN** una estación lista
- **WHEN** el cliente invoca el endpoint de promoción
- **THEN** recibe el `node_id`, la ruta del seed, el indicador `created`, el contenido y las fuentes
