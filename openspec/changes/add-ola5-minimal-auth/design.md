## Context

See proposal.md — Why. El gateway HTTP es la frontera del prototipo web; el
router gRPC sigue siendo alcanzable en lab. Ambos leen `AVLP_SESSION_SECRET`
para activar el mismo modo seguro.

## Goals / Non-Goals

**Goals:** token HMAC, metadata gRPC, IDOR cerrado en progress/station, promote
solo teacher, UI mínima, compatibilidad open-mode.

**Non-Goals:** usuarios persistentes, refresh tokens, TLS mTLS, RBAC fino.

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

## Risks / Trade-offs

- **[Risk]** gRPC directo sin metadata en modo seguro → Unauthenticated.
- **[Risk]** Secret débil → documentar longitud mínima recomendada (≥32 chars).
- **[Trade-off]** Sin DB de sesiones: revocación = rotar el secret.

## Migration Plan

Desplegar gateway+router con el mismo secret. Open mode por default.
