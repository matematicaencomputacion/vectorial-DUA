# Delta Spec: Hierarchical Subtopic Node

## ADDED Requirements

### Requirement: Árbol de subtemas opcionales

El sistema SHALL permitir adjuntar un `DUAHierarchicalTree` a un nodo interactivo con subtemas opcionales recursivos.

#### Scenario: Seed automóvil válido

- GIVEN el seed `automovil-hierarchy.json`
- WHEN se carga en el registry
- THEN el nodo tiene `hierarchy.macro_media_url` no vacío
- AND existen subtemas opcionales con hijos COMPONENT

### Requirement: Navegación no lineal

El sistema SHALL resolver un subtema por id sin exigir haber visitado hermanos.

#### Scenario: Path a Motor sin Asientos

- GIVEN el árbol automóvil
- WHEN se llama `PathTo("sub_motor")`
- THEN el path incluye Caja Central y Motor
- AND no requiere Asientos

### Requirement: Registro de interacción

El sistema SHALL registrar toques de subtema vía `RecordSubtopicInteraction`.

#### Scenario: HasOpened

- GIVEN un estudiante registra apertura de `sub_motor`
- WHEN se consulta `HasOpened(student, parent, sub_motor)`
- THEN retorna true
- AND `HasOpened(..., sub_asientos)` retorna false
