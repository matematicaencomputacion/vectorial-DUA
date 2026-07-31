## Context

See proposal.md — Why. El **gateway HTTP es el único punto de verificación** del
token HMAC en el prototipo web. El router gRPC **confía** en la metadata que el
gateway inyecta (`avlp-student-id`, `avlp-role`, `avlp-auth-mode`). Ambos leen
`AVLP_SESSION_SECRET` para activar el mismo modo seguro (el router lo usa solo
para decidir si exige metadata).

## Goals / Non-Goals

**Goals:** token HMAC, metadata gRPC, IDOR cerrado en progress/station, promote
solo teacher, UI mínima, compatibilidad open-mode, bind por defecto en loopback.

**Non-Goals:** usuarios persistentes, refresh tokens, TLS mTLS entre gateway y
router, RBAC fino, validación del token HMAC dentro del router.

## Decisions

### 1. Token compacto propio (sin dependencia JWT)

Formato `v1.<b64url(json)>.<b64url(hmac-sha256)>`. Claims: `sid`, `role`, `exp`.
TTL default 24h (`AVLP_SESSION_TTL`). Evita librerías nuevas.

### 2. Precedencia del student_id

En modo seguro: metadata `avlp-student-id` gana. Si el body/query trae otro ID →
`NotFound` (sin filtrar existencia). En modo abierto: body/query como hoy.

### 3. Promote

Modo seguro: metadata `avlp-role=teacher` obligatoria; si no →
`PermissionDenied` con mensaje neutro. Teacher se obtiene solo si
`AVLP_TEACHER_KEY` coincide en `POST /api/session`. Sin key configurada, nadie
es teacher. Modo abierto: promote sin rol (prototipo).

### 4. UI

Token solo en memoria. `sessionStorage` sigue solo para student_id de
conveniencia local (como hoy). Dev panel muestra `secure_mode` y `role`.

### 5. Frontera de confianza: loopback por defecto

El router bindea por defecto en `127.0.0.1:50051`. `AVLP_ROUTER_ADDR` permite
abrirlo explícitamente (p. ej. `:50051` o una IP de red). En modo seguro, si la
dirección efectiva **no** es loopback, el router loguea una advertencia clara:
exponerlo fuera de loopback sin mTLS permite fabricar metadata y suplantar
identidad/rol. Modelo: **gateway verifica; router confía; router en loopback
(o detrás de mTLS)**.

## Risks / Trade-offs

- **[Risk]** Cliente gRPC directo que fabrica metadata → mitiga bind loopback +
  warning; no cierra el riesgo si se abre a la red a propósito.
- **[Risk]** gRPC directo sin metadata en modo seguro → Unauthenticated.
- **[Risk]** Secret débil → documentar longitud mínima recomendada (≥32 chars).
- **[Trade-off]** Sin DB de sesiones: revocación = rotar el secret.
- **[Debt — Ola 6]** Alternativa robusta: que el router valide el token HMAC él
  mismo, o mTLS entre gateway y router (además de usuarios persistentes /
  OAuth / más roles).

## Migration Plan

Desplegar gateway+router con el mismo secret. Open mode por default. Router en
loopback salvo override explícito de `AVLP_ROUTER_ADDR`.
