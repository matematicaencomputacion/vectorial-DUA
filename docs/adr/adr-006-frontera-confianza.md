# ADR-006 — Frontera de confianza: gateway-verifica / router-confía

**Estado:** Aceptado · **Fecha:** 2026-08-03 · **Autores:** destilado Ola 5 minimal-auth

## Contexto

`PromoteLiveStation` y el resto de RPCs necesitan identidad (student/teacher)
sin montar aún un IdP completo. El prototipo HTTP debe poder abrirse en lab y
cerrarse con sesión HMAC.

## Decisión

- **Gateway (`master-web` / `webgateway`)** verifica la sesión HMAC cuando
  `AVLP_SESSION_SECRET` está seteado; emite rol `teacher` solo con
  `AVLP_TEACHER_KEY`.
- **Router gRPC** confía en metadata de identidad que le llega del gateway.
- El router **debe** escuchar en loopback por defecto
  (`AVLP_ROUTER_ADDR=127.0.0.1:50051`). Si el bind no es loopback, se loguea
  WARNING de riesgo de suplantación **sin mTLS**.

### Limitación registrada

Cualquier cliente que alcance el puerto gRPC puede fabricar metadata y
suplantar identidad. El modelo es deliberado para el prototipo.

### Evolución (deuda)

Validar el token HMAC **dentro del router**, o **mTLS** gateway↔router
(documentado como deuda Ola 6).

## Consecuencias

- Un solo lugar verifica cookies/tokens HTTP.
- Despliegues expuestos deben cambiar el modelo antes de salirse de loopback.
- Spec OpenSpec `minimal-auth` fija el contrato.

## Referencias

- README «Sesión y confianza» + deuda mTLS: `README.md`.
- WARNING loopback: `cmd/router/main.go` (bind no-loopback sin mTLS).
- Spec: `openspec/specs/minimal-auth/spec.md`
  («Router en loopback o mTLS»).
- Design + no-goals mTLS: `openspec/changes/archive/2026-07-31-add-ola5-minimal-auth/design.md`.
- Tasks deuda Ola 6: `…/add-ola5-minimal-auth/tasks.md` §4.2.
- Gateway verifica Bearer → metadata gRPC: `pkg/webgateway/gateway.go`
  (`authedContext`).
- Metadata saliente / no revalida HMAC en router: `pkg/session/context.go`
  (`RequireSecureIdentity`, `AppendOutgoingMetadata`).
- Session FromEnv: `pkg/session/session.go`.
