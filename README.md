# AVLP — Adaptive Vector Learning Platform (vectorial-DUA)

[![CI](https://github.com/matematicaencomputacion/vectorial-DUA/actions/workflows/ci.yml/badge.svg)](https://github.com/matematicaencomputacion/vectorial-DUA/actions/workflows/ci.yml)

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

Las estaciones en vivo quedan indexadas (quien repite una duda novel vuelve a su estación en lugar de pagar una nueva). Para que no eclipsen al material curado —el que trae botonera, schemas DUA y pedagogía revisada—, un nodo live gana el matching estático solo si supera al mejor curado por más de `vector.LivePreferenceMargin` (**0.05**); ante empate o diferencia menor gana el curado. Además, los nodos live vencen perezosamente según `AVLP_LIVE_NODE_TTL` (duración Go, default **24h**); los curados no expiran.

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

Una estación `ready` que un docente revisó puede ascender a currículo persistente:

```text
PromoteLiveStation(tracking_ulid)
POST /api/stations/{tracking_ulid}/promote
  → seed data/nodes/interactive/promoted-{tracking_ulid}.json
  → mismo node_id, ahora curado e idempotente
```

El seed conserva markdown, fuentes y embedding; por eso vuelve a cargarse después de reiniciar el router. El directorio respeta `AVLP_INTERACTIVE_NODES_DIR`.

Ejemplo de flujo completo:

```bash
go run ./cmd/router
go run ./cmd/router-client -mode poll -student demo-student
```

## Prototipo web

UI Master de laboratorio (Stage ~70% + botonera ~30%) sobre el gateway HTTP/JSON. No es producción: es fiel al contrato interactivo y al camino de estaciones pendientes.

```bash
go run ./cmd/router                 # gRPC :50051
go run ./cmd/master-web             # http://127.0.0.1:8080  (AVLP_WEB_ADDR / AVLP_ROUTER_ADDR)
```

Flujo esperado en texto:

```text
1. Escribís una duda (o usás un chip de ejemplo) → POST /api/query
2a. Match + has_interactive_payload → GET /api/nodes/{id}
    Stage muestra stage_media_default; botonera renderiza
    schema (depth|cognitive|emergency|combined), legacy y/o hierarchy
2b. Pending → pantalla de espera con student_message rogeriano
    poll GET /api/stations/{ulid}?student_id=… cada 2s
    ready → live_content (Markdown simple) en el Stage
3. Toque de botonera/subtema → POST /api/interactions/…
4. “+ Tengo una duda diferente” → POST /api/nodes/{id}/mutate
    el botón LIVE aparece en la botonera sin recargar la página
```

Checklist manual (teclado + lector de pantalla + voz): `cmd/master-web/MANUAL_CHECKLIST.md`.

**Trust model del prototipo.** Los RPCs y el gateway confían en el `student_id`
declarado por el cliente — igual que `RecordSubtopicInteraction` /
`RecordBotoneraInteraction` y `GetSubtopicProgress`. La promoción confía en la
posesión del `tracking_ulid`; una fase multi-usuario requiere autenticación,
autorización por estudiante y rol docente para promover.

Dictado por voz (mejora progresiva): si el navegador expone Web Speech API, aparece un micrófono junto a «Tu duda» y a «+ Tengo una duda diferente» (`lang: es-AR`, sin auto-enviar). Atajo **Ctrl+M** en el campo principal. Soportado en Chrome/Edge (probado en Chrome); si no hay API, el botón no se renderiza.

El miss path RAG descarta hits con similitud &lt; `AVLP_RAG_MIN_SIMILARITY` (default **0.30**). La simmatrix query×chunk documenta el compromiso de cada embedder: con hash normalizado, el caso on-topic más débil queda ~0.36 y el control off-topic «qué es un bit» ~0.24 (piso sugerido ~0.30). Sin hits queda el camino honesto («No encontré material verificado…»); con embedders densos el piso debe recalibrarse con datos.

## Ciclo docente

Cuando una estación live sale bien, el docente puede adoptarla como currículo:

```text
generar (miss → estación live) → revisar (contenido + fuentes) → promover
```

```bash
# La estación quedó ready (poll o UI). Promoción:
curl -X POST http://127.0.0.1:8080/api/stations/{tracking_ulid}/promote
# → seed data/nodes/interactive/promoted-{tracking_ulid}.json (idempotente)
```

El seed conserva markdown, fuentes y embedding; el mismo `node_id` pasa a curado y sobrevive reinicios. TTL live (`AVLP_LIVE_NODE_TTL`) no afecta a los promovidos.

## Configuración (`AVLP_*`)

| Variable | Default | Rol |
|----------|---------|-----|
| `AVLP_SIMILARITY_THRESHOLD` | — (ver abajo) | Override del umbral coseno estático (0–1] |
| `AVLP_CONFIG_PATH` | `data/avlp.json` en el router | Archivo JSON de umbral (`calibrate --apply`) |
| `AVLP_EMBEDDING_URL` | vacío → hash offline | Base OpenAI-compatible (`…/v1`) |
| `AVLP_EMBEDDING_MODEL` | `text-embedding-3-small` | Modelo remoto |
| `AVLP_EMBEDDING_API_KEY` | vacío | Bearer opcional |
| `AVLP_EMBEDDING_DIMS` | descubrimiento | Dims fijas del cliente HTTP |
| `AVLP_EMBEDDING_TIMEOUT` | `10s` | Timeout HTTP del embedder |
| `AVLP_LLM_URL` | vacío → fallback extractivo | Base Chat Completions compatible (`…/v1`) |
| `AVLP_LLM_MODEL` | `qwen3:4b-instruct` | Modelo de síntesis generativa |
| `AVLP_LLM_API_KEY` | vacío | Bearer opcional del backend LLM |
| `AVLP_LLM_TIMEOUT` | `30s` | Timeout total de síntesis |
| `AVLP_RAG_MIN_SIMILARITY` | `0.30` | Piso coseno de hits RAG |
| `AVLP_RAG_ENABLED` | `true` | Materializar estaciones live vía RAG |
| `AVLP_KB_ROOT` | `data/knowledge_base` | Raíz de la base documental |
| `AVLP_STATION_TTL` | `24h` | TTL del ledger de estaciones pendientes |
| `AVLP_LIVE_NODE_TTL` | `24h` | TTL de nodos live en el índice k-NN |
| `AVLP_INTERACTIVE_NODES` | `true` | Carga de seeds Stage/botonera |
| `AVLP_INTERACTIVE_NODES_DIR` | `data/nodes/interactive` | Directorio de seeds (incluye promovidos) |
| `AVLP_PROFILE_STORE_PATH` | vacío → memoria | Snapshot JSON de perfiles $V_e$ |
| `AVLP_ROUTER_ADDR` | `:50051` | Bind gRPC del router / dial del gateway |
| `AVLP_WEB_ADDR` | `127.0.0.1:8080` | Bind HTTP de `master-web` |

**Precedencia del umbral estático:** request gRPC válido > `AVLP_SIMILARITY_THRESHOLD` > archivo (`AVLP_CONFIG_PATH` / `data/avlp.json`) > `0.85`. El router loguea valor y origen al arrancar.

## Síntesis LLM local (Ollama)

Sin `AVLP_LLM_URL`, las estaciones usan el renderer extractivo y el router lo
informa en logs. Para explicaciones generativas 100% locales:

```bash
ollama pull qwen3:4b-instruct

export AVLP_LLM_URL=http://localhost:11434/v1
export AVLP_LLM_MODEL=qwen3:4b-instruct
go run ./cmd/router
```

Recomendación base: [`qwen3:4b-instruct`](https://ollama.com/library/qwen3/tags)
por su soporte multilingüe/español y tamaño cuantizado de ~2.5 GB, razonable
para un Mac mini con memoria limitada. Con 16 GB o más, `qwen3:8b` (~5.2 GB Q4)
mejora calidad a cambio de latencia y memoria.

El modelo recibe el `FullPrompt` grounded (contexto RAG + tono rogeriano +
formato DUA). Debe usar únicamente ese contexto; la aplicación agrega `Fuentes`
al final y, ante timeout/error, registra la causa y vuelve explícitamente al
modo extractivo. La revisión docente sigue siendo obligatoria antes de promover.

## Embeddings

Por defecto el router y el harness usan `HashEmbedder` offline (64 dims, léxico). Con URL remota se activa un cliente HTTP OpenAI-compatible (`POST …/embeddings`) sin fallback silencioso a hash.

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
go run ./cmd/harness -suite simmatrix -embedder env  # query×nodo + query×chunk RAG
go run ./cmd/harness -suite calibrate -embedder env --apply
go run ./cmd/harness -suite evals -embedder env
```

`calibrate` sugiere umbral = punto medio entre la peor similitud correcta y la mejor incorrecta (margen a cada lado; aviso si margen &lt; 0.05). `--apply` lo escribe atómicamente en `data/avlp.json` (o `-config <ruta>`).

Antes de embeber, tanto queries como descriptores de seeds y chunks pasan por la misma normalización: minúsculas, eliminación de diacríticos y colapso de letras consecutivas repetidas. El embedder hash también descarta palabras funcionales frecuentes para que no inflen similitudes off-topic. Esto mantiene simétrico el espacio de búsqueda; el caso real «variables y escopes» forma parte de la matriz y de los tests de ruteo/RAG.

## Perfiles del estudiante ($V_e$)

Por defecto el router guarda $V_e$ en memoria. Con `AVLP_PROFILE_STORE_PATH` (p. ej. `data/profiles.json`) el snapshot es versionado (`version`, `ve_dims`, `profiles`), con escritura atómica y flush debounced (~1s); `Close()` al shutdown hace flush final. Misismatch de `ve_dims` o JSON corrupto se descartan con log (arranque vacío, sin crash).

## Árbol del repositorio

```text
vectorial-DUA/
├── cmd/{router,router-client,harness,master-web}
├── internal/{routerserver,testenv}
├── pkg/{vector,rag,livestation,dua,rogerian,webgateway}
├── data/knowledge_base/
├── data/nodes/interactive/
├── proto/ + gen/
├── harness/{evals,sandbox,load,telemetry}
├── .github/workflows/ci.yml
├── scripts/{test-clean.sh,gen-proto.sh}
└── openspec/
    ├── specs/{routing,routing-robustness,rag,interactive_node,
    │         live-node-lifecycle,harness}/
    └── changes/archive/
        ├── 2026-07-29-add-ola3-master-web/          (v0.3.1-ola3b)
        ├── 2026-07-29-add-ola3-subtopic-progress/   (v0.3.2-ola3c)
        ├── 2026-07-30-add-ola4-quality-audit/       (Ola 4.b)
        ├── 2026-07-30-add-ola4-live-node-policy/    (Ola 4.c)
        └── 2026-07-30-add-ola4-routing-robustness/  (Ola 4.d — tag v0.4.0)
```

## Guía de ejecución

```powershell
go test ./...
go run ./cmd/router
go run ./cmd/harness -suite evals
```

Regenerar Protobuf: `./scripts/gen-proto.ps1`

### Tests y variables de entorno

El ruteo lee `AVLP_*` en tiempo de ejecución (umbral de similitud, embedder, pisos
de RAG, toggles de nodos interactivos). Un shell con `AVLP_SIMILARITY_THRESHOLD=0.55`
exportado puede volver verde una regresión real, así que los tests fijan su propio
entorno vía `internal/testenv`: `testenv.Isolate(t)` por test y `testenv.Clear()`
desde `TestMain` en los paquetes que rutean. El helper limpia por prefijo, de modo
que cubre variables nuevas sin actualizar una lista.

Antes de un push, correr la suite sin nada exportado (`env | grep AVLP` vacío):

```bash
./scripts/test-clean.sh
```
