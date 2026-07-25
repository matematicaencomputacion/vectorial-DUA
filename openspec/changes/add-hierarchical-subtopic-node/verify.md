# Verification: add-hierarchical-subtopic-node

Fecha: 2026-07-25

| Escenario | Resultado | Evidencia |
|-----------|-----------|-----------|
| Seed automóvil + hierarchy | PASS | `TestHierarchySeedLoadAndPath` |
| Path no lineal (Motor) | PASS | `PathTo("sub_motor")` |
| Ids únicos / profundidad | PASS | `TestHierarchyRejectsDuplicateIDs` |
| Record interacción | PASS | `TestInteractionStoreRecord` |
| Hierarchy-only Validate | PASS | `TestValidateAllowsHierarchyOnly` |
| Build router | PASS | `go build ./cmd/router` |

```powershell
go test ./pkg/dua/...
go build ./cmd/router
```
