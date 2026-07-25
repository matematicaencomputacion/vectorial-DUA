# Delta Spec: Esquemas de Botonera DUA

## ADDED Requirements

### Requirement: Esquema de profundidad temática

El sistema SHALL permitir botoneras tipadas con variantes de profundidad (express, core, deep_dive, under_the_hood) y duración estimada.

#### Scenario: Nodo depth con 4 variantes

- GIVEN un seed con `botonera_schema.kind=depth`
- WHEN se valida el nodo
- THEN existen al menos las variantes express y core
- AND cada variante tiene `media_url` y `duration_seconds > 0`

### Requirement: Esquema de estilos cognitivos

El sistema SHALL soportar variantes de formato cognitivo (animation, live_coding, diagram, debate).

#### Scenario: Validación live_coding

- GIVEN una variante `live_coding`
- WHEN se valida
- THEN incluye `media_url` o `cell_code`

### Requirement: Esquema de emergencia

El sistema SHALL soportar botones de diagnóstico (error, hint rogeriano, test cases) sin revelar la solución completa en hints.

#### Scenario: Hint sin solución

- GIVEN una variante `hint`
- WHEN se valida
- THEN `hint_text` no está vacío
- AND no incluye un campo de solución completa

### Requirement: Esquema combinado profundidad × formato

El sistema SHALL modelar una matriz de celdas indexadas por ejes de profundidad y formato.

#### Scenario: Celda de matriz resoluble

- GIVEN ejes depth=[express,standard] y format=[video,diagram]
- AND cells cubriendo el producto cartesiano o un subconjunto declarado
- WHEN el cliente selecciona (express, video)
- THEN existe una celda con esos ids y `media_url` no vacío
