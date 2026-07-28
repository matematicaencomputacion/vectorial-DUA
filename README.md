# AVLP — Adaptive Vector Learning Platform (vectorial-DUA)

Plataforma educativa vectorial y adaptativa (**DUA** + **Carl Rogers**), con **OpenSpec (SDD)**, **Harness**, **RAG** y **nodos interactivos** (Stage + botonera).

## Triángulo del entorno adaptativo

| Capa | Rol | Estado en repo |
|------|-----|----------------|
| **Master** | Mapa de rutas DUA + nodos interactivos | Contrato Stage/botonera + seeds |
| **IDE (Antigravity)** | Ejecución y experimentación en celdas | Sandbox + acción `open_ide_cell` |
| **Agente** | Scaffolding rogeriano + RAG; actualiza $V_e$ por interacción | mutación “duda diferente”; deltas vía `RecordBotoneraInteraction` / `RecordSubtopicInteraction` |

## Nodo interactivo DUA (Stage + botonera)

Layout multipartes inspirado en reproductores interactivos clásicos (Dreamweaver/Flash), formalizado como `InteractiveVideoNode`:

```text
+---------------------------------------------------+---------------------+
|           REPRODUCTOR CENTRAL (STAGE) ~70%        |  BOTONERA DUA ~30%  |
|     clip / diagrama según botón activo            |  esquemas tipados   |
+---------------------------------------------------+---------------------+
```

### Esquemas reutilizables de botonera

| Kind | Uso | Seed |
|------|-----|------|
| `depth` | Zoom express / core / deep / under_the_hood | `variables-scope.json` |
| `cognitive` | Animación, live coding, diagrama, debate | `async-cognitive.json` |
| `emergency` | Error, hint Rogers, casos de prueba | `debug-emergency.json` |
| `combined` | Matriz profundidad × formato | `postgis-combined.json` |

- RPCs: `GetInteractiveNode`, `MutateInteractiveNode`, `RecordBotoneraInteraction`, `RecordSubtopicInteraction`
- Flag: `AVLP_INTERACTIVE_NODES` (default `true`)
- Contrato: `DUANodeBotonera` + `BotoneraInteraction` (preferencias → $V_e$)

### Accordion Vectorial / subtemas opcionales

Un `InteractiveVideoNode` puede adjuntar `hierarchy` (`DUAHierarchicalTree`): nodo raíz con macro media + subtemas recursivos opcionales (`SubtopicNode`, profundidad MACRO → COMPONENT → MICRO).

- Navegación **no lineal**: el estudiante puede abrir “Motor” sin pasar por “Asientos”.
- `orbit_delta` sitúa el subtema respecto al embedding del padre (sin entradas k-NN separadas en esta fase).
- `RecordSubtopicInteraction` registra toques en memoria (`InteractionStore`) para que el Agente sepa qué ramas ya vio.
- Seed: `data/nodes/interactive/automovil-hierarchy.json` (“¿Qué es un Automóvil?” → Caja Central / 4 Ruedas).
- Compatibilidad: `botonera_schema` y/o `hierarchy` en el mismo nodo; el Stage muestra macro o el clip del subtema activo.

## Flujo RAG

```text
Duda → Router k-NN
  ├─ sim ≥ 0.85 → Nodo DUA (estático o interactivo)
  └─ sim < 0.85 → RAG → Estación en vivo (ULID)
```

En un nodo interactivo, **“+ Tengo una duda diferente”** llama a `MutateInteractiveNode` y appende un botón `is_live_generated=true`.

## Árbol del repositorio

```text
vectorial-DUA/
├── cmd/{router,router-client,harness}
├── pkg/{vector,rag,livestation,dua,rogerian}
├── data/knowledge_base/
├── data/nodes/interactive/
├── proto/ + gen/
├── harness/
├── openspec/changes/
│   ├── init-adaptive-vector-router/
│   ├── init-harness-and-vector-router/
│   ├── add-rag-knowledge-retriever/
│   ├── add-interactive-video-node/
│   ├── add-dua-botonera-schemas/
│   ├── add-hierarchical-subtopic-node/
│   └── add-ola2-adaptive-debt/   (deuda Ola 2, propuesta)
└── scripts/
```

## Guía de ejecución

```powershell
go test ./...
go run ./cmd/router
go run ./cmd/harness -suite evals
```

Regenerar Protobuf: `./scripts/gen-proto.ps1`
