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

## Ciclo de vida de estaciones pendientes

Cuando el miss no puede materializar la estación al instante (RAG off/fallo), `QueryNearestNode` devuelve `LiveStationPending` con `tracking_ulid` y un **mensaje rogeriano en español** (no diagnóstico técnico). El router registra la solicitud en `StationLedger` (`in_progress` | `ready` | `failed`, TTL `AVLP_STATION_TTL`, default 24h).

El camino de retorno es `GetLiveStation(tracking_ulid, student_id)`:

```text
QueryNearestNode → pending(tracking_ulid, message)
        │
        ▼
GetLiveStation (poll) ──► in_progress (sigue el mensaje rogeriano)
        │                 ready → node_id + live_content + sources
        │                 failed → student_message (sin causa interna)
        └─ ULID ausente o student_id incorrecto → NotFound (mismo código; sin filtrar existencia)
```

Lazy retry: si el generator ya está disponible, el poll puede disparar un único `GenerateLive`. Un flag `retrying` bajo lock evita generaciones duplicadas ante polls concurrentes.

Ejemplo de flujo completo:

```bash
go run ./cmd/router
go run ./cmd/router-client -mode poll -student demo-student
```

## Embeddings

Por defecto el router y el harness usan `HashEmbedder` offline (64 dims, léxico). Con URL remota se activa un cliente HTTP OpenAI-compatible (`POST …/embeddings`) sin fallback silencioso a hash.

| Variable | Rol |
|----------|-----|
| `AVLP_EMBEDDING_URL` | Base del endpoint (p. ej. `http://localhost:11434/v1`); si está vacía → modo offline |
| `AVLP_EMBEDDING_MODEL` | Modelo (default `text-embedding-3-small`) |
| `AVLP_EMBEDDING_API_KEY` | Bearer opcional |
| `AVLP_EMBEDDING_DIMS` | Dims fijas; si se omite, se descubren en el primer call |
| `AVLP_EMBEDDING_TIMEOUT` | Timeout HTTP (default `10s`) |
| `AVLP_SIMILARITY_THRESHOLD` | Umbral coseno (0–1) cuando el request no trae umbral; default `0.85` |

`0.85` está calibrado para hash/plumbing. Con **bge-m3** (Ollama, 1024 dims) la calibración con `simmatrix` cerró en umbral **`0.55`**:

| Señal (bge-m3) | Rango de referencia |
|----------------|---------------------|
| Match correcto (paráfrasis → nodo esperado) | 0.665–0.765 |
| Fuera de manifold (novel / live) | ≤ 0.386 |
| Brecha static−live | ~0.28 |
| Umbral validado | **0.55** |

La discriminación entre nodos del mismo tema usa *doc-expansion* (preguntas típicas embebidas). Re-calibrá al crecer el corpus:

```bash
export AVLP_SIMILARITY_THRESHOLD=0.55
go run ./cmd/harness -suite simmatrix -embedder env    # matriz query×nodo
go run ./cmd/harness -suite calibrate -embedder env  # → harness/out/calibration.json
go run ./cmd/harness -suite evals -embedder env
```

`calibrate` sugiere umbral = punto medio entre la peor similitud correcta y la mejor incorrecta (margen a cada lado; aviso si margen &lt; 0.05). El índice usa las dims del embedder activo. Evals CI usan hash.

## Perfiles del estudiante ($V_e$)

Por defecto el router guarda $V_e$ en memoria. Para persistir entre reinicios:

| Variable | Rol |
|----------|-----|
| `AVLP_PROFILE_STORE_PATH` | Ruta al snapshot JSON (p. ej. `data/profiles.json`); vacía → in-memory |

El snapshot es versionado (`version`, `ve_dims`, `profiles`). Escritura atómica con flush debounced (~1s); `Close()` al shutdown hace flush final. Misismatch de `ve_dims` o JSON corrupto se descartan con log (arranque vacío, sin crash).

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
│   ├── add-ola2-adaptive-debt/   (deuda Ola 2, saldada)
│   └── add-ola3-station-ledger/  (Ola 3.a / C2 cerrada — tag v0.3.0-ola3a)
└── scripts/
```

## Guía de ejecución

```powershell
go test ./...
go run ./cmd/router
go run ./cmd/harness -suite evals
```

Regenerar Protobuf: `./scripts/gen-proto.ps1`
