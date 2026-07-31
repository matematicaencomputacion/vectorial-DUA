## Why

Hoy todo RPC confía en el `student_id` que declara el cliente, y
`PromoteLiveStation` —que escribe curriculum— no tiene barrera. Es la deuda M4
arrastrada desde Ola 3.c y el prerequisito mínimo de multi-usuario.

## What Changes

- Sesión HMAC emitida por el gateway (`POST /api/session`) firmada con
  `AVLP_SESSION_SECRET` (vacío → modo abierto, documentado y logueado).
- Token Bearer con `student_id`, rol (`student`|`teacher`) y expiración.
- Gateway valida el token y propaga identidad por metadata gRPC; precedencia del
  ID verificado sobre el body/query.
- En modo seguro: `GetLiveStation` / `GetSubtopicProgress` aíslan por token
  (IDOR → NotFound); `PromoteLiveStation` exige rol `teacher` vía
  `AVLP_TEACHER_KEY`.
- UI: sesión al cargar (token en memoria); campo plegado «Soy docente»; botón
  promover solo con rol teacher.

### Incluido

- Tests (gateway + routerserver) y Playwright (abierto + seguro).
- README reemplazando la nota de trust model; OpenSpec.

### Fuera de alcance

- Base de usuarios, passwords, OAuth, recuperación de claves, roles extra
  (deuda Ola 6).

### Rollback

Vaciar `AVLP_SESSION_SECRET` restaura el modo abierto del prototipo.

## Capabilities

### New Capabilities

- `minimal-auth`: Sesión HMAC, roles student/teacher, aislamiento IDOR y
  promoción protegida.

### Modified Capabilities

Ninguna.

## Impact

`pkg/session`, `pkg/webgateway`, `internal/routerserver`, `cmd/master-web`,
`cmd/router`, README, tests y verify Playwright.
