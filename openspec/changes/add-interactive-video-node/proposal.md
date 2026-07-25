# Proposal: Nodo de Video Interactivo DUA (Stage + Botonera)

## Problema

Los videos lineales largos generan fatiga y abandono. El alumno no puede saltar a la atomización exacta de su duda ("¿qué es?", "error común", "ejemplo"), ni enriquecer el nodo cuando la botonera no cubre su bloqueo puntual.

## Objetivo

Formalizar el dominio `InteractiveVideoNode` con layout multipartes (Stage central + botonera DUA), contratos Protobuf/Go, seed de ejemplo y mutación en vivo: `[ + Tengo una duda diferente ]` → RAG + Agente Rogeriano → nuevo botón dinámico.

## Alcance incluido

- OpenSpec change `add-interactive-video-node`.
- `proto/interactive_node.proto` + RPC `MutateInteractiveNode` / `GetInteractiveNode`.
- Tipos y validación en `pkg/dua`.
- Registry in-memory + seed JSON.
- Mutación vía RAG reutilizando `pkg/rag`.
- Documentación del contrato de layout 70/30 para futuro Master UI.

## Fuera de alcance

- UI React/Tailwind del Master.
- CDN real de videos.
- Persistencia relacional de botones mutados.

## Riesgos

- URLs placeholder no reproducibles sin assets.
- Mutaciones concurrentes sobre la misma botonera requieren locking (RWMutex en registry).

## Plan de rollback

- Feature flag `AVLP_INTERACTIVE_NODES=false` desactiva carga/registry y RPCs responden Unimplemented/empty.
