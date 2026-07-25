# Technical Design: Botoneras estructurales DUA

## Cuatro esquemas

| Kind | Propósito |
|------|-----------|
| `depth` | Zoom pedagógico: express / core / deep / under_the_hood |
| `cognitive` | Estilos: animación, live coding, diagrama, debate |
| `emergency` | Bloqueos: error, hint Rogers, casos de prueba |
| `combined` | Matriz profundidad × formato |

## Protobuf

`DUANodeBotonera.kind` selecciona el esquema activo. Los ejes `depth_axis` / `format_axis` + `matrix_cells` modelan la botonera combinada.

`InteractiveVideoNode.botonera` (plana) se mantiene por compatibilidad; si `botonera_schema` está presente, el Master la prioriza.

## Preferencias de perfil

`BotoneraInteraction` registra `schema_kind`, `variant_id`, `format_type` y un `preference_delta` opcional para desplazar $V_e$ (ritmo / sensorial).

## Layout combinado (contrato UI)

```text
STAGE
[ Profundidad ] Express | Estándar | Profundo
[ Formato     ] Video   | Código   | Diagrama
```
