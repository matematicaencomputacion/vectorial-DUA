# Proposal: Nodo Jerárquico de Subtemas DUA (Accordion Vectorial)

## Problema

Los nodos interactivos actuales (Stage + botonera) no modelan el scaffolding progresivo por agregación de detalles: un mapa macro opcionalmente desplegable en micro-nodos (árbol fractálico), con navegación no lineal y registro de qué subtemas tocó el estudiante.

## Objetivo

Formalizar `DUAHierarchicalTree` / `SubtopicNode` recursivo adjunto a `InteractiveVideoNode`, con opcionalidad, path no lineal, y RPC `RecordSubtopicInteraction` para preferencia de detalle en $V_e$.

## Alcance incluido

- OpenSpec `add-hierarchical-subtopic-node`.
- Protobuf recursivo + Ack RPC.
- Dominio Go (Validate, FindByID, PathTo, InteractionStore).
- Seed automóvil (Caja Central → Asientos/Volante/Motor; 4 Ruedas).
- Wiring en `cmd/router`.

## Fuera de alcance

- UI accordion React.
- Persistencia DB.
- Obligatoriedad de consumir todos los subtemas.

## Rollback

- Nodos sin `hierarchy` siguen válidos; RPC responde FailedPrecondition si store deshabilitado.
