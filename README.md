# AVLP — Adaptive Vector Learning Platform (vectorial-DUA)

[![CI](https://github.com/matematicaencomputacion/vectorial-DUA/actions/workflows/ci.yml/badge.svg)](https://github.com/matematicaencomputacion/vectorial-DUA/actions/workflows/ci.yml)

Plataforma educativa vectorial y adaptativa (**DUA** + **Carl Rogers**), con **OpenSpec (SDD)**, **Harness**, **RAG** y **nodos interactivos** (Stage + botonera).

## Documentación

- Runbook Neo4j (GCP / IAP / sync): [docs/neo4j-gcp.md](docs/neo4j-gcp.md)
- Decisiones de arquitectura: [ADR-001 — criterio de lenguajes C++/Go](docs/adr-001-criterio-lenguajes.md)
- Deuda / backlog Ola 7: [docs/ola7-backlog.md](docs/ola7-backlog.md)

## Grafo de conocimiento

Currículum dirigido en git (`data/knowledge/curriculum.json`): conceptos con
slug estable (`concept:<slug>`) y aristas tipadas
(`requires` / `deepens` / `continues` / `alternative`). La fuente de verdad es
**git** (Pedagogy-as-Code); Neo4j es réplica de lectura opcional.

- **Orientación en la UI:** tras cargar un nodo, el rail «Para ubicarte»
  (`orientation.js`) pide `GET /api/nodes/{id}/orientation` sin candados; el
  copy sale del Advisor rogeriano (visitas locales por estudiante).
- **graph-sync:** `go run ./cmd/graph-sync` valida con `LoadFile` antes de
  cualquier escritura Bolt (`-dry-run`, `-prune`, `-validate-seeds`); idempotente
  con `MERGE` + `synced_at`.
- **Runbook GCP / IAP:** [docs/neo4j-gcp.md](docs/neo4j-gcp.md).

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

Cuando el miss no puede materializar la estación al instante (RAG off/fallo), o cuando la generación supera `AVLP_LLM_SYNC_DEADLINE` (default **2s**), `QueryNearestNode` devuelve `LiveStationPending` con `tracking_ulid` y un **mensaje rogeriano en español** (no diagnóstico técnico). El router registra la solicitud en `StationLedger` (`in_progress` | `ready` | `failed`, TTL `AVLP_STATION_TTL`, default 24h) y sigue generando en segundo plano.

Si el mismo embedding vuelve a matchear un nodo `live://stations/{ulid}` ya listo, la respuesta matched rehidrata `live_content` y fuentes desde el ledger (el Stage no debería mostrar solo la URL).

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
go run ./cmd/router                 # gRPC 127.0.0.1:50051 (loopback por defecto)
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

**Sesión y confianza.** Con `AVLP_SESSION_SECRET` vacío el prototipo sigue en
**modo abierto** (el cliente declara `student_id`; se loguea al arrancar). Con
secret configurado el **gateway es el único punto de verificación**: emite un
token HMAC (`POST /api/session`), las APIs HTTP exigen `Authorization: Bearer`,
y propaga identidad por metadata gRPC (`avlp-student-id`, `avlp-role`). El
**router confía en esa metadata** (no revalida el token). Precedencia: el
`student_id` del token gana sobre body/query; si difieren → `NotFound` (sin
filtrar existencia). `PromoteLiveStation` en modo seguro exige rol `teacher`,
obtenido solo si `AVLP_TEACHER_KEY` coincide al pedir sesión. Sin esa clave,
nadie puede promover en modo seguro.

Por eso el router bindea por defecto en **loopback** (`127.0.0.1:50051`).
`AVLP_ROUTER_ADDR` puede abrirlo a otras interfaces a propósito; en modo seguro
se loguea una advertencia si no es loopback (sin mTLS, un cliente gRPC directo
puede fabricar metadata y suplantar rol). El router debe permanecer en loopback
o detrás de mTLS.

Deuda Ola 6: que el router valide el token él mismo o mTLS gateway↔router;
además usuarios persistentes, passwords/OAuth y más roles.

Dictado por voz (mejora progresiva): cascada **STT local** (si `AVLP_STT_URL`) →
**Web Speech API** → sin botón. Con STT local, `voice.js` graba con MediaRecorder
y envía a `POST /api/transcribe` (sin auto-enviar; tope 60 s). Atajo **Ctrl+M**
en el campo principal. El panel de desarrollo muestra `voice: mode=…`.
Errores de cada camino son visibles en español.

### STT local en Mac (recomendado)

La Web Speech API manda audio a la nube (Google) y en lab falló por red. Para
dictado **sin nube**, levantá un servidor OpenAI-compatible
`/v1/audio/transcriptions`. En Apple Silicon conviene **whisper.cpp**
(`whisper-server`) con modelo **`ggml-small`** (o `base` si priorizás RAM):
buen equilibrio tamaño/calidad para español, ~500 MB el small.

```bash
# Ejemplo con binario whisper-server (ajustá rutas a tu build/homebrew):
./whisper-server -m models/ggml-small.bin --host 127.0.0.1 --port 8081

export AVLP_STT_URL='http://127.0.0.1:8081/v1'
# opcionales: AVLP_STT_MODEL=whisper-1  AVLP_STT_LANGUAGE=es  AVLP_STT_TIMEOUT=30s
# AVLP_STT_API_KEY=… si el servidor lo exige

go run ./cmd/router
go run ./cmd/master-web
```

Alternativa: `faster-whisper-server` u otro compatible con el mismo path.
Sin `AVLP_STT_URL`, la UI cae a Web Speech si el navegador lo expone.

El miss path RAG descarta hits con similitud &lt; `AVLP_RAG_MIN_SIMILARITY` (default **0.30**). La simmatrix query×chunk documenta el compromiso de cada embedder: con hash normalizado, el caso on-topic más débil queda ~0.36 y el control off-topic «qué es un bit» ~0.24 (piso sugerido ~0.30). Sin hits queda el camino honesto («No encontré material verificado…»); con embedders densos el piso debe recalibrarse con datos.

## Ciclo docente

Cuando una estación live sale bien, el docente puede adoptarla como currículo:

```text
generar (miss → estación live) → revisar (contenido + fuentes) → promover
```

```bash
# Modo seguro (gateway + router con el mismo secret):
export AVLP_SESSION_SECRET='cambia-esto-por-un-secreto-largo'
export AVLP_TEACHER_KEY='clave-instituto'
# UI: plegado «Soy docente» → re-emite token teacher → botón promover

# La estación quedó ready (poll o UI). Promoción (requiere teacher en modo seguro):
curl -H "Authorization: Bearer $TOKEN" -X POST http://127.0.0.1:8080/api/stations/{tracking_ulid}/promote
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
| `AVLP_LLM_SYNC_DEADLINE` | `2s` | Ventana sync del miss path; al vencer → `pending` + poll. `0` = siempre asíncrono. Con modelos locales (p. ej. Ollama) conviene un valor alto (`30s`–`60s`) o `0` |
| `AVLP_RAG_MIN_SIMILARITY` | `0.30` | Piso coseno de hits RAG |
| `AVLP_RAG_ENABLED` | `true` | Materializar estaciones live vía RAG |
| `AVLP_KB_ROOT` | `data/knowledge_base` | Raíz de la base documental |
| `AVLP_STATION_TTL` | `24h` | TTL del ledger de estaciones pendientes |
| `AVLP_LIVE_NODE_TTL` | `24h` | TTL de nodos live en el índice k-NN |
| `AVLP_REQUIRE_MEDIA_A11Y` | `false` | Si `true`, rechaza seeds con huecos de accesibilidad de medios |
| `AVLP_SESSION_SECRET` | vacío → modo abierto | Secret HMAC de sesión; vacío mantiene el prototipo abierto |
| `AVLP_SESSION_TTL` | `24h` | Expiración del token de sesión |
| `AVLP_TEACHER_KEY` | vacío | Clave de instituto para emitir rol `teacher` (solo modo seguro) |
| `AVLP_STT_URL` | vacío → cascada Web Speech / sin mic | Base OpenAI-compatible del STT (`…/v1`); habilita dictado local |
| `AVLP_STT_MODEL` | `whisper-1` | Modelo enviado al endpoint de transcripción |
| `AVLP_STT_API_KEY` | vacío | Bearer opcional del STT |
| `AVLP_STT_TIMEOUT` | `30s` | Timeout HTTP del transcriber |
| `AVLP_STT_LANGUAGE` | `es` | Código de idioma para el STT |
| `AVLP_INTERACTIVE_NODES` | `true` | Carga de seeds Stage/botonera |
| `AVLP_INTERACTIVE_NODES_DIR` | `data/nodes/interactive` | Directorio de seeds (incluye promovidos) |
| `AVLP_PROFILE_STORE_PATH` | vacío → memoria | Snapshot JSON de perfiles $V_e$ |
| `AVLP_KNOWLEDGE_GRAPH_PATH` | `data/knowledge/curriculum.json` | Currículum JSON (fuente de verdad en git) |
| `AVLP_KNOWLEDGE_STRICT` | vacío/`false` | Si `true`, avisos graduales (concepto sin recurso, etc.) fallan la carga |
| `AVLP_CONCEPT_STORE_PATH` | vacío → memoria | Persistencia de visitas a conceptos por estudiante |
| `AVLP_NEO4J_URI` | vacío → off | Bolt URI read-only del currículum (`neo4jgraph`); vacío = MemoryGraph archivo |
| `AVLP_NEO4J_USER` / `AVLP_NEO4J_PASSWORD` | vacío | Auth básica Bolt |
| `AVLP_NEO4J_COOLDOWN` | `30s` | Ventana del breaker tras 3 fallos (log una vez por ventana) |
| `AVLP_KNOWLEDGE_CACHE_TTL` | `5m` | Caché TTL de lecturas Neo4j |
| `AVLP_ROUTER_ADDR` | `127.0.0.1:50051` | Bind gRPC del router (loopback) / dial del gateway; override explícito para otras interfaces |
| `AVLP_WEB_ADDR` | `127.0.0.1:8080` | Bind HTTP de `master-web` |

**Precedencia del umbral estático:** request gRPC válido > `AVLP_SIMILARITY_THRESHOLD` > archivo (`AVLP_CONFIG_PATH` / `data/avlp.json`) > `0.85`. El router loguea valor y origen al arrancar.

La tabla cubre **sesión** (`AVLP_SESSION_*`, `AVLP_TEACHER_KEY`), **STT local** (`AVLP_STT_*`) y **grafo / Neo4j** (`AVLP_KNOWLEDGE_*`, `AVLP_NEO4J_*`, `AVLP_CONCEPT_STORE_PATH`). Sin `AVLP_STT_URL` la UI cae a Web Speech o no muestra micrófono; sin `AVLP_SESSION_SECRET` el prototipo permanece en modo abierto; sin `AVLP_NEO4J_URI` la orientación usa solo el MemoryGraph archivo.

## Accesibilidad de medios (DUA — múltiples medios de representación)

El contrato interactivo admite alternativas opcionales en el nodo raíz y en cada
variante/subtema:

| Campo | Rol |
|-------|-----|
| `alt_text` | Descripción breve para lectores de pantalla |
| `transcript` | Texto plano o Markdown equivalente al clip |
| `captions_url` | WebVTT de subtítulos |
| `audio_description_url` | Pista narrada opcional |

`Validate()` **no** exige estos campos (curriculum y promovidos siguen cargando).
`AccessibilityReport` lista huecos en video/audio sin alternativa textual; al
cargar seeds, `Registry.LoadDir` los loguea. Con `AVLP_REQUIRE_MEDIA_A11Y=true`
esos huecos fallan la carga — para institutos que ya puedan exigir el estándar.

En el Stage: se muestra el `alt_text`; si hay `transcript`, un control «Ver
transcripción» (símbolo + texto, `aria-expanded`, teclado); si el medio es
`<video>` y hay `captions_url`, se agrega `<track kind="captions">`. Al promover
una estación live, el markdown pasa a `transcript` del nodo curado.

## Síntesis LLM local (Ollama)

Sin `AVLP_LLM_URL`, las estaciones usan el renderer extractivo y el router lo
informa en logs. Para explicaciones generativas 100% locales:

```bash
ollama pull qwen3:4b-instruct

export AVLP_LLM_URL=http://localhost:11434/v1
export AVLP_LLM_MODEL=qwen3:4b-instruct
# Con modelos locales la síntesis suele superar el default 2s:
#   AVLP_LLM_SYNC_DEADLINE=0     → pending inmediato + poll (recomendado en lab)
#   AVLP_LLM_SYNC_DEADLINE=45s   → esperar más antes de cortar a async
go run ./cmd/router

# Verificación opcional (fuera de CI; TestMain limpia AVLP_*):
RUN_LLM_INTEGRATION=1 go test ./pkg/livestation -run GenerateWithConfiguredLLMIntegration -v
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
├── cmd/{router,router-client,harness,master-web,graph-sync}
├── internal/{routerserver,testenv}
├── pkg/{vector,rag,livestation,dua,rogerian,webgateway,knowledge}
├── data/knowledge_base/
├── data/knowledge/
├── data/nodes/interactive/
├── docs/{neo4j-gcp.md,adr-001-criterio-lenguajes.md,ola7-backlog.md}
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
