## ADDED Requirements

### Requirement: Evidencia de visitas por estudiante

El sistema SHALL persistir visitas a conceptos por `student_id` sin filtrar
visitas de un estudiante a otro.

#### Scenario: Aislamiento

- **GIVEN** el estudiante A visitó `concept:env-file` y B no
- **WHEN** se aconseja a B sobre un concepto que requiere `env-file`
- **THEN** B recibe gap sobre `env-file` y A no

### Requirement: Advisor rogeriano sin jerga

`Advisor.Advise` SHALL devolver copy en español usando títulos y rationale,
sin jerga interna (DUA, grafo, Neo4j, `concept:`, KnowledgeGraph).

#### Scenario: Gap único

- **GIVEN** un foco con prerrequisito no visitado y rationale curado
- **WHEN** se llama `Advise`
- **THEN** `MessageES` menciona el título del prerrequisito y no contiene jerga
