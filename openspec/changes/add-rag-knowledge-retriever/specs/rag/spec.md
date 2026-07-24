# Delta Spec: RAG Knowledge Retriever

## ADDED Requirements

### Requirement: Indexación de knowledge base en chunks vectoriales

El sistema SHALL fragmentar documentos Markdown de `data/knowledge_base/` e indexarlos con embeddings en un índice k-NN en memoria.

#### Scenario: Ingest exitoso de documentos seed

- GIVEN una carpeta `data/knowledge_base` con al menos un `.md`
- AND un `Embedder` configurado
- WHEN se ejecuta el pipeline de ingest
- THEN cada chunk tiene texto no vacío, `source` path y embedding de dimensión fija
- AND el índice de chunks reporta `Len() > 0`

### Requirement: Recuperación top-k ante miss del router

El sistema SHALL, cuando ningún nodo pedagógico alcanza similitud ≥ 0.85, recuperar los k chunks más cercanos por coseno.

#### Scenario: Retrieve con query alineada a un documento

- GIVEN knowledge base indexada que incluye contenido sobre variables de entorno
- WHEN se consulta con un embedding/texto alineado a ese tema
- THEN el retriever devuelve al menos un chunk cuyo `source` referencia ese documento
- AND la similitud del top-1 es mayor que la de chunks no relacionados

### Requirement: Estación en vivo anclada a contexto RAG + tono Rogers

El sistema SHALL sintetizar una estación en vivo usando solo el contexto recuperado y plantillas rogerianas, persistiendo un nodo ULID con `is_live_generated=true`.

#### Scenario: Miss materializa nodo live con sources

- GIVEN un query embedding sin nodo estático ≥ 0.85
- AND RAG habilitado con knowledge base cargada
- WHEN el cliente llama `QueryNearestNode`
- THEN la respuesta es `matched` con `is_live_generated=true`
- AND `retrieved_sources` no está vacío
- AND el nodo queda registrado en el índice pedagógico

#### Scenario: RAG deshabilitado conserva pending

- GIVEN `AVLP_RAG_ENABLED=false`
- WHEN ocurre un miss de ruteo
- THEN el sistema devuelve `LiveStationPending`
- AND emite `NodeNotFoundEvent`

### Requirement: Evaluación de fidelidad del RAG

El harness SHALL puntuar faithfulness y context relevance de las estaciones generadas.

#### Scenario: Respuesta anclada al contexto pasa eval

- GIVEN una estación sintetizada a partir de chunks conocidos
- WHEN el scorer de RAG calcula faithfulness y relevance
- THEN el aggregate ≥ 0.80
- AND el caso se marca PASS
