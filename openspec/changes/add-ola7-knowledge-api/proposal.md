## Why

El Advisor de Ola 7.2 no debe vivir dentro de `QueryNearestNode`. La UI necesita
consultar orientación después del render y rutas de aprendizaje bajo demanda.

## What Changes

- `proto/knowledge.proto` + RPCs `GetNodeOrientation` / `GetConceptRoute`.
- Handlers con auth e aislamiento por `student_id`; `available=false` OK si no
  hay Advisor.
- Gateway: `GET /api/nodes/{id}/orientation` y `GET /api/concepts/route`.

## Impact

`proto/`, `internal/routerserver`, `pkg/webgateway`, `scripts/gen-proto.sh`.
