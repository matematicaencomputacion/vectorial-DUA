## ADDED Requirements

### Requirement: Orientación por RPC dedicado

El sistema SHALL exponer `GetNodeOrientation` que usa el Advisor y NEVER deberá
mezclar `advice_es` en `QueryNearestNode`.

#### Scenario: Sin advisor

- **GIVEN** router sin Advisor
- **WHEN** `GetNodeOrientation`
- **THEN** responde OK con `available=false`

### Requirement: Ruta entre conceptos

`GetConceptRoute` SHALL devolver orden de aprendizaje o `available=false` ante
falla de transporte.

#### Scenario: Camino fundacional

- **GIVEN** env-secrets requiere … string-literals
- **WHEN** route from env-secrets to string-literals
- **THEN** el primer id es string-literals
