# Spec: Knowledge Graph

## Purpose

Grafo dirigido de conceptos curriculares versionado en git (`concept:<slug>` +
aristas tipadas), con binding a recursos DUA y validación dura en carga.

## Requirements

### Requirement: Grafo de conceptos versionado

El sistema SHALL cargar un grafo dirigido de conceptos (`concept:<slug>`) y
aristas tipadas desde JSON versionado, con validación dura y avisos graduales.

#### Scenario: Ciclo en requires

- **GIVEN** un archivo con ciclo A→B→A en `requires`
- **WHEN** se llama `LoadFile`
- **THEN** falla e imprime el ciclo concreto

### Requirement: Binding concepto–recurso

Los nodos interactivos y demo SHALL declarar `concepts`; el binder SHALL
resolver `ResourcesFor` / `ConceptsForNode` en arranque.

#### Scenario: Concepto enseñado

- **GIVEN** un seed que declara `concept:env-file`
- **WHEN** se consulta `ResourcesFor(env-file)`
- **THEN** incluye ese recurso
