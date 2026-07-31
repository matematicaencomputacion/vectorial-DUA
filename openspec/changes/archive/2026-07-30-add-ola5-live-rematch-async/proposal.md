## Why

Tras generar una estación live, el re-match k-NN encuentra el nodo
`live://stations/{ulid}` pero no rehidrata el markdown desde el ledger: el Stage
muestra la URL. Además, con síntesis LLM el miss path bloquea 10–30s y invita a
búsquedas encimadas aunque el polling de Ola 3.a ya exista.

## What Changes

- Al matchear un nodo live, `QueryNearest` rellena `live_content` (y fuentes)
  desde `StationLedger` usando el ULID de `resource_url`.
- En el miss path, la generación puede cortar a asíncrono tras
  `AVLP_LLM_SYNC_DEADLINE`: responde `LiveStationPending` de inmediato y el
  polling existente trae la estación cuando el modelo termina.
- Tests de regresión del rematch y verificación Playwright del Stage con
  contenido (no solo la URL).

### Incluido

- Hidratación server-side (router → gRPC ya mapea `LiveContent`).
- Deadline configurable, default que deja el camino extractivo en sync.
- Documentación de la variable nueva.

### Fuera de alcance

- Cambios de Protobuf, streaming de tokens, cola persistente de jobs.
- Auth multi-usuario, promoción docente, TTL del índice.
- Frontend: no se inventa contenido; solo consume `live_content` como hoy.

### Rollback

Sin cambios de contrato: quitar la hidratación vuelve al bug visible; poner
`AVLP_LLM_SYNC_DEADLINE` muy alto (p. ej. `5m`) restaura el miss casi siempre
síncrono. El ledger y el poll no cambian de esquema.

## Capabilities

### New Capabilities

- `live-station-delivery`: Entrega de contenido live en rematch y generación
  asíncrona acotada por deadline en el miss path.

### Modified Capabilities

Ninguna.

## Impact

Afecta `pkg/vector` (router + ledger), tests de estación, README (`AVLP_*`),
y `cmd/master-web/verify`. `internal/routerserver` solo se beneficia del
outcome enriquecido; no requiere lógica nueva de hidratación.
