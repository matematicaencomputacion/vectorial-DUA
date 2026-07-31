## ADDED Requirements

### Requirement: Sesión firmada opcional

El gateway SHALL emitir un token HMAC en `POST /api/session` cuando
`AVLP_SESSION_SECRET` está configurado. Si el secret está vacío, SHALL operar
en modo abierto (comportamiento actual) y loguearlo al arrancar.

#### Scenario: Emisión en modo seguro

- **GIVEN** un secret de sesión configurado
- **WHEN** el cliente pide sesión con un `student_id`
- **THEN** recibe un Bearer token con rol `student` y expiración

#### Scenario: Rol docente

- **GIVEN** `AVLP_TEACHER_KEY` configurada y una clave correcta en la solicitud
- **WHEN** se emite la sesión
- **THEN** el token lleva rol `teacher`

#### Scenario: Modo abierto

- **GIVEN** secret vacío
- **WHEN** se pide sesión
- **THEN** la respuesta indica `secure_mode=false` y no exige Bearer en las APIs

### Requirement: Frontera de confianza en el gateway

En modo seguro el gateway SHALL validar `Authorization: Bearer`, preferir el
`student_id` verificado sobre body/query, y propagarlo por metadata gRPC.

#### Scenario: Precedencia

- **GIVEN** un token de estudiante A y un body que declara estudiante B
- **WHEN** se consulta progreso o estación
- **THEN** el sistema no expone datos de B (`NotFound`)

### Requirement: Promoción solo con rol teacher

En modo seguro, `PromoteLiveStation` SHALL exigir rol `teacher`; en caso
contrario SHALL devolver `PermissionDenied` con mensaje neutro.

#### Scenario: Estudiante intenta promover

- **GIVEN** modo seguro y un token `student`
- **WHEN** solicita promoción
- **THEN** recibe denegación sin detalles internos

### Requirement: Metadata requerida en el routerserver

Cuando el router opera en modo seguro, SHALL exigir metadata de autenticación
en los RPCs sensibles; su ausencia SHALL mapear a `Unauthenticated`.

#### Scenario: Llamada gRPC sin metadata

- **GIVEN** el router con `AVLP_SESSION_SECRET` configurado
- **WHEN** un cliente invoca un RPC sensible sin metadata de sesión
- **THEN** recibe `Unauthenticated`
