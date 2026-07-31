# Spec: Live Station Delivery

## Purpose

Asegura que el contenido de una estación live esté disponible tanto en el miss
path como al re-matchear el nodo indexado, y que la generación larga no bloquee
indefinidamente el RPC de consulta.

## Requirements

### Requirement: Rematch rehidrata el contenido desde el ledger

Cuando `QueryNearest` matchea un nodo live cuyo `resource_url` identifica una
estación (`live://stations/{tracking_ulid}`), el outcome SHALL incluir el
contenido y las fuentes almacenados en `StationLedger` si la estación está
`ready`.

#### Scenario: Segunda consulta de la misma duda

- **GIVEN** una estación live lista registrada en el ledger y su nodo en el índice
- **WHEN** una consulta posterior matchea ese nodo por similitud
- **THEN** la respuesta matched incluye `live_content` no vacío
- **AND** conserva `is_live_generated=true` y las fuentes recuperadas

#### Scenario: Ledger ausente o expirado

- **GIVEN** un nodo live en el índice sin registro ready en el ledger
- **WHEN** se matchea ese nodo
- **THEN** la respuesta matched sigue devolviendo el nodo
- **AND** no inventa `live_content`

### Requirement: Miss path puede completar en asíncrono tras un deadline

El router SHALL iniciar la generación live sin bloquear el RPC más allá de
`AVLP_LLM_SYNC_DEADLINE` (default 2s). Si la generación no termina a tiempo,
SHALL devolver `LiveStationPending` y completar vía ledger + poll.

#### Scenario: Generación rápida dentro del deadline

- **GIVEN** un miss con generator disponible que termina antes del deadline
- **WHEN** se consulta el nearest
- **THEN** la respuesta es matched con el contenido de la estación

#### Scenario: Generación lenta supera el deadline

- **GIVEN** un miss cuya generación supera `AVLP_LLM_SYNC_DEADLINE`
- **WHEN** se consulta el nearest
- **THEN** la respuesta es pending con `tracking_ulid`
- **AND** cuando la generación termina el ledger pasa a `ready`
- **AND** `GetLiveStation` / el polling existente expone el contenido

#### Scenario: Deadline cero

- **GIVEN** `AVLP_LLM_SYNC_DEADLINE=0`
- **WHEN** ocurre un miss con generator
- **THEN** la respuesta pending se devuelve de inmediato mientras la generación sigue
