## Why

La calibración actual solo informa un umbral y el router no puede consumirlo
sin una variable exportada. Además, queries, seeds y chunks no comparten una
normalización explícita, y la matriz existente no muestra el espacio RAG donde
se decide si una fuente es suficientemente relevante.

## What Changes

- `calibrate --apply` persiste atómicamente el umbral sugerido en configuración.
- El router resuelve y registra el umbral con precedencia env > archivo > default.
- Todos los embedders normalizan query, descriptor y chunk en el mismo límite.
- `simmatrix` agrega una matriz query×chunk y evidencia para calibrar
  `AVLP_RAG_MIN_SIMILARITY`, incluido «variables y escopes» → `.env`.

### Incluido

- Config JSON local y versionada, normalización Unicode, matrices y tests.
- Documentación del origen efectivo y de solapamientos que impiden un corte
  limpio con el embedder hash.

### Fuera de alcance

- Aplicar automáticamente el piso RAG sugerido.
- Corrección ortográfica general o cambio de modelo de embeddings.
- Persistencia remota/distribuida de configuración.

### Rollback

Eliminar `data/avlp.json` (o desactivar `AVLP_CONFIG_PATH`) restaura el default;
`AVLP_SIMILARITY_THRESHOLD` puede fijar inmediatamente un valor conocido. La
normalización se revierte retirándola del límite `Embed`, sin migrar datos
persistentes porque el índice se reconstruye al arrancar.

## Capabilities

### New Capabilities

- `routing-robustness`: Configuración calibrada, normalización simétrica y
  diagnóstico query×nodo/query×chunk del espacio de similitud.

### Modified Capabilities

Ninguna.

## Impact

Afecta `cmd/harness`, `cmd/router`, `pkg/vector`, `pkg/rag`, `harness/evals`,
descriptores de seeds, documentación y tests. Agrega uso directo de
`golang.org/x/text/unicode/norm`, ya presente transitivamente.
