# Delta Spec: Harness de Control AVLP

## ADDED Requirements

### Requirement: Evaluación de calidad de ruteo pedagógico DUA

El sistema SHALL ejecutar una suite de evals que puntúe recomendaciones del motor vectorial según señales DUA/Rogers, no solo similitud coseno.

#### Scenario: Caso golden con match estático correcto

- GIVEN un dataset golden con query embedding y dimensión DUA esperada
- AND un índice seed con el nodo esperado
- WHEN el harness de evals ejecuta el caso
- AND el router devuelve un nodo estático con similitud ≥ 0.85
- AND la dimensión DUA coincide con la expectativa
- THEN el caso obtiene score agregado ≥ 0.80
- AND se registra como PASS en el EvalReport

#### Scenario: Caso golden que exige estación en vivo

- GIVEN un query embedding sin nodos cercanos (similitud &lt; 0.85)
- WHEN el harness ejecuta el caso con outcome esperado `live`
- THEN el router emite fallback `NodeNotFound` / pending tracking ULID
- AND el caso PASS si no se fuerza un nodo estático incorrecto

### Requirement: Sandbox de ejecución de celdas de estudiante

El sistema SHALL ejecutar código de estudiante solo bajo política de sandbox con timeout y límites de salida.

#### Scenario: Violación por timeout

- GIVEN una celda que excede el timeout configurado
- WHEN el sandbox executor la corre
- THEN la ejecución se cancela
- AND el resultado marca `Violation=timeout`
- AND no deja procesos huérfanos controlados por el harness

### Requirement: Load harness gRPC con percentiles

El sistema SHALL medir latencia p50/p95/p99 y tasa de error bajo concurrencia gRPC configurable.

#### Scenario: Carga nominal

- GIVEN el router escuchando en `:50051`
- AND N workers concurrentes con M requests totales
- WHEN el load harness finaliza
- THEN reporta QPS, error_rate y percentiles
- AND falla la suite si error_rate ≥ 1%

### Requirement: Telemetría y trazas LLM

El sistema SHALL registrar métricas de ruteo/evals y spans de llamadas LLM con latencia y propósito.

#### Scenario: Export de snapshot

- GIVEN métricas acumuladas durante una corrida de harness
- WHEN se solicita export
- THEN se escribe un TelemetrySnapshot serializable (JSON)
- AND incluye contadores y spans LLM si hubieran ocurrido
