# Verification: add-dua-botonera-schemas

Fecha: 2026-07-25

| Escenario | Resultado | Evidencia |
|-----------|-----------|-----------|
| 4 seeds tipados | PASS | `TestBotoneraSchemasLoad` |
| Depth express+core | PASS | seed `variables-scope.json` |
| Hint requiere texto | PASS | `TestHintRequiresText` |
| Matriz express×video | PASS | `ResolveCombined` |
| Build router | PASS | `go build ./cmd/router` |

```powershell
go test ./pkg/dua/...
```
