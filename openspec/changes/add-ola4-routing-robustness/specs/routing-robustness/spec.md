## Purpose

Hace reproducible la calibración del ruteo y mantiene consultas, contenido
curado y fuentes RAG dentro de un espacio de embeddings normalizado y medible.

## ADDED Requirements

### Requirement: Umbral calibrado persistente

El harness SHALL poder persistir el umbral sugerido y el router SHALL resolver
el umbral efectivo con precedencia variable de entorno, archivo y default.

#### Scenario: Aplicar una calibración

- **GIVEN** una ejecución de calibración con umbral sugerido válido
- **WHEN** el operador usa `calibrate --apply`
- **THEN** el sistema escribe atómicamente un archivo de configuración versionado

#### Scenario: La variable de entorno prevalece

- **GIVEN** un archivo válido y `AVLP_SIMILARITY_THRESHOLD` válido
- **WHEN** arranca el router
- **THEN** usa el valor de la variable y registra su origen como `env`

#### Scenario: Fallback determinista

- **GIVEN** una variable ausente o inválida y un archivo ausente o inválido
- **WHEN** arranca el router
- **THEN** usa el default `0.85` y registra su origen como `default`

### Requirement: Normalización simétrica para embeddings

El sistema SHALL aplicar la misma normalización de minúsculas, diacríticos y
letras consecutivas repetidas a todo texto en el límite de embedding.

#### Scenario: Query y descriptor equivalentes

- **GIVEN** una query y un descriptor que difieren solo en mayúsculas, tildes o letras repetidas
- **WHEN** ambos se embeben
- **THEN** producen representaciones del mismo texto normalizado

#### Scenario: Caso real de typo

- **GIVEN** la consulta `variables y escopes` y los seeds interactivos
- **WHEN** se ejecuta el ruteo
- **THEN** el nodo de Variables y Scope resulta el destino estático más cercano

### Requirement: Diagnóstico RAG por chunk

La suite `simmatrix` SHALL puntuar cada query de calibración contra todos los
chunks RAG y SHALL informar piso actual, piso sugerido, margen y advertencias.

#### Scenario: Referencia env

- **GIVEN** la consulta `variables y escopes` y la base de conocimiento
- **WHEN** se construye la matriz query×chunk
- **THEN** la fila identifica los chunks `env-variables` como fuente esperada

#### Scenario: Solapamiento no oculto

- **GIVEN** evidencia on-topic y off-topic sin separación positiva
- **WHEN** se calcula la sugerencia
- **THEN** el reporte conserva los puntajes y emite una advertencia de solapamiento
