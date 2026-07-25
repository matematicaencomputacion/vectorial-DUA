# Technical Design: Accordion Vectorial / Subtemas Opcionales

## UX (contrato Master)

```text
NODO RAIZ: macro media (45s)
Navegador subtemas:
  [Caja Central] desplegado
      Asientos | Volante | Motor
  [4 Ruedas] no tocado
```

Reglas: opcionalidad, no linealidad, registro de toques → Agente recomienda subtemas no vistos ante errores.

## Datos

`InteractiveVideoNode.hierarchy = DUAHierarchicalTree`

Subtemas con `orbit_delta` (Δv conceptual respecto al padre). Max profundidad default 3 (`MACRO`/`COMPONENT`/`MICRO`).

## RPC

`RecordSubtopicInteraction(SubtopicInteraction) → Ack`

Store in-memory: `(student_id, parent_node_id) → set(subtopic_ids)`.
