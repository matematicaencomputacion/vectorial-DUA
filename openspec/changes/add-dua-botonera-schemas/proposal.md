# Proposal: Esquemas reutilizables de Botonera DUA

## Problema

La botonera plana de `InteractiveButton` no expresa patrones pedagógicos reutilizables (profundidad, estilo cognitivo, emergencia, matriz). El Frontend y el perfil vectorial del estudiante necesitan esquemas tipados para registrar preferencias (express vs deep, video vs diagrama).

## Objetivo

Definir 4 esquemas estructurales de botonera DUA como contratos Protobuf/Go reutilizables, compatibles con el Stage interactivo existente, y con seeds de ejemplo.

## Alcance incluido

- OpenSpec `add-dua-botonera-schemas`.
- Extensión de `interactive_node.proto`: `DepthVariant`, estilos cognitivos, emergencia, matriz combinada, `DUANodeBotonera`.
- Validación Go + seeds JSON por tipo.
- Evento ligero `BotoneraInteraction` para actualizar el vector de preferencias (contrato).

## Fuera de alcance

- UI React del Master.
- Analytics warehouse / persistencia de preferencias.

## Rollback

- Nodos sin `botonera_schema` siguen usando `botonera` plana (compatibilidad).
