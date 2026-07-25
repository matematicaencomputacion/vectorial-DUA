# Delta Spec: Interactive Video Node DUA

## ADDED Requirements

### Requirement: Servir nodo interactivo con layout Stage + botonera

El sistema SHALL exponer nodos con `layout_type=interactive_dashboard` y una botonera de acciones DUA atomizadas.

#### Scenario: GetInteractiveNode devuelve botonera

- GIVEN un seed `InteractiveVideoNode` cargado en el registry
- WHEN el cliente llama `GetInteractiveNode` con el `node_id`
- THEN la respuesta incluye `stage_media_default` y al menos un botón en `botonera`
- AND cada botón PLAY_CLIP tiene `media_url` o timestamps válidos

### Requirement: Acciones de botonera tipadas

El sistema SHALL soportar acciones `PLAY_CLIP`, `OPEN_IDE_CELL` y `ASK_AGENT` en la botonera.

#### Scenario: Botón open_ide_cell con código

- GIVEN un botón con `action_type=OPEN_IDE_CELL`
- WHEN se valida el nodo
- THEN `cell_code` no está vacío

### Requirement: Mutación en vivo por duda diferente

El sistema SHALL, ante una duda no cubierta, recuperar contexto RAG y appender un botón dinámico a la botonera.

#### Scenario: MutateInteractiveNode crea botón live

- GIVEN un nodo interactivo existente
- AND una knowledge base RAG indexada
- WHEN el cliente llama `MutateInteractiveNode` con `doubt_text`
- THEN se añade un botón con `is_live_generated=true`
- AND el botón tiene `vector_delta` no vacío
- AND `GetInteractiveNode` refleja la botonera enriquecida
