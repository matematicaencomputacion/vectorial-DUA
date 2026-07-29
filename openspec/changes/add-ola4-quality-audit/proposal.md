# Proposal: Auditoría dura de mantenibilidad (Ola 4.b)

**Estado:** solo plan — **no implementar** en este change.  
**Base:** `main` @ `aa56a41` (post merge CI / PR #5).  
**Método:** skill `thermo-nuclear-code-quality-review` (Cursor Team Kit) sobre el árbol completo, no un diff de PR.  
**Agente:** [thermo-nuclear](8fbbaf8b-62cb-4728-9f23-3b5e5e0ebca4).

## Contexto

AVLP en este HEAD es un prototipo cohesivo por capas (`pkg/{vector,dua,rag,livestation,webgateway,rogerian}` + `cmd/{router,master-web,harness}`) con paquetes Go en su mayoría por debajo de ~300 líneas y buena separación dominio/transporte. La superficie que incumple el umbral de salud del rubric (~1k líneas) y concentra deuda estructural es la UI Master monolítica (`cmd/master-web/web/index.html` ≈ **1826** líneas), acompañada de acumulación de handlers gRPC en `cmd/router/main.go` (~501, 7 RPCs) y de un espejo deliberado del algoritmo de progreso de subtemas en JS. El resto del árbol es manejable; conviene atacar pocos hallazgos de alta convicción antes de que Ola 4 sume políticas live, calibrate `--apply` y más RPC.

Esta salida alimenta la priorización conjunta revisor + Dario: qué entra a 4.c / 4.d, qué se pospone, y qué se descarta con justificación.

---

## Hallazgos por severidad

### Critical

#### C1 — `index.html` supera ~1k líneas y concentra CSS + markup + app JS

- **Ubicación:** `cmd/master-web/web/index.html` (~1826; ~580 CSS, ~85 markup, ~1147 JS IIFE, ~48 funciones).
- **Por qué importa:** Viola la regla no negociable del rubric (no dejar un archivo crecer injustificadamente más allá de ~1k). Un solo archivo es el stage, la botonera (legacy/tabs/matriz/jerarquía), voz, polling de estación, progreso, API client, markdown y guards de generación (`isStale` aparece decenas de veces). Cada Ola (3.b → 3.c → 4) añade ramas al mismo flujo ocupado: crecimiento spaghetti, no “un prototipo compacto”.
- **Dirección recomendada:** Descomponer **sin framework** (ver temas especiales). Objetivo: shell HTML delgado + CSS + módulos JS por responsabilidad; `//go:embed web/*` sigue válido. No migrar a React/Vue en Ola 4.

### High

#### H1 — Doble implementación del agregado de progreso sin harness de parity

- **Ubicación:** `pkg/dua/subtopic_progress.go` (`ProgressForTree` / `progressInSubtree`); `cmd/master-web/web/index.html` (`subtreeProgress`, `deriveProgress`, `updateHierarchyProgress`, `loadProgressForNode`); tests solo en Go (`subtopic_progress_test.go`).
- **Por qué importa:** El diseño canónico (Ola 3.c) define total, intersección pre-order y estados de raíz en Go. El cliente **reescribe** el mismo walk y, tras `GET /progress`, descarta `root_states` del servidor y re-deriva con `deriveProgress(..., opened_ids)`. El UI además necesita estado por **cada** nodo del acordeón (Go solo expone raíces), así que la duplicación no es accidental: es deuda de contrato. Cualquier cambio de semántica (`visited`/`partial`/`unvisited`, IDs desconocidos, orden) puede divergir sin que CI lo note.
- **Dirección recomendada:** Propiedad a largo plazo en temas especiales; plan mínimo: fixtures JSON compartidos + parity (Go test + smoke JS / golden Playwright), o enriquecer el RPC para anotar estados por nodo y dejar al cliente solo el set optimista de `opened`.

#### H2 — Handlers gRPC canónicos en `cmd/router` y espejo casi completo en tests del gateway

- **Ubicación:** `cmd/router/main.go` (7 métodos `(*server)` + bootstrap `main`); `pkg/webgateway/gateway_test.go` (`inProcessRouter`, 7 métodos, comentario explícito “mirrors…”).
- **Por qué importa:** Cada RPC nuevo se implementa dos veces. El harness de test ya diverge en detalles (frustración por defecto, embedder fake, mensajes). A ~501 líneas el archivo aún no cruza 1k, pero la **acumulación** + duplicación es el riesgo real ante Ola 4 (auth, política live, más RPCs).
- **Dirección recomendada:** Extraer un paquete único de servidor (p. ej. `internal/routerserver`) con el tipo `Server` + handlers; `cmd/router` solo cablea deps y `Listen`; tests del gateway/router usan la misma implementación. No fragmentar un archivo por RPC todavía.

#### H3 — Guards de generación / stale esparcidos como spaghetti de control flow

- **Ubicación:** `cmd/master-web/web/index.html` (`bumpLoadGeneration` / `isStale` / parámetro `gen` en cargas, poll, voz, submit; ~33 usos).
- **Por qué importa:** El comportamiento (invalidar cargas en vuelo) es correcto y necesario, pero el modelo de cancelación está **inlined** en cada rama async. Un flujo nuevo obliga a recordar el ritual o reintroducir races. Refuerza C1.
- **Dirección recomendada:** Al descomponer, encapsular en un `LoadSession` / `RequestGeneration` (módulo `session.js`): `begin()`, `token()`, `run(async fn)` que no-op si stale.

### Medium

#### M1 — Adaptador `LiveGenerator` triplicado

- **Ubicación:** `cmd/router/main.go` (`liveBridge`); `cmd/harness/main.go` (`harnessLiveBridge`); `harness/evals/evals_test.go` (`liveBridge`).
- **Por qué importa:** Tres wrappers idénticos `livestation.Request` ↔ `vector.LiveRequest`.
- **Dirección:** Que `livestation.Generator` implemente `vector.LiveGenerator` (o un `AsLiveGenerator()` único). Borrar los tres adapters.

#### M2 — `InteractionStore` vive en `hierarchy.go` (cohesión incorrecta)

- **Ubicación:** `pkg/dua/hierarchy.go` (tipos de árbol + store mutable de progreso/perfil).
- **Por qué importa:** Mezcla modelo de jerarquía (puro/validación) con tracking de sesión. El progreso puro ya está en `subtopic_progress.go`.
- **Dirección:** Mover a `interaction_store.go` (mismo package `dua`). Mecánico, bajo riesgo.

#### M3 — `playwright-check.mjs` crece como segundo monolito de verificación

- **Ubicación:** `cmd/master-web/verify/playwright-check.mjs` (~405; modos `chips|progress|routerdown|full`).
- **Por qué importa:** Si Ola 4 suma auth/live TTL, seguirá el destino de `index.html`.
- **Dirección:** Partir por modo + runner mínimo; no bloquear features 4.c/4.d, sí acotar crecimiento.

#### M4 — Confianza en `student_id` declarado por el cliente

- **Ubicación:** comentario en `GetSubtopicProgress`; `openspec/changes/add-ola4-live-node-policy` ítem 4.
- **Por qué importa:** Deuda de frontera ya documentada; si Ola 4 toca aislamiento live sin auth, el modelo mental sigue siendo mentiroso.
- **Dirección:** Auth como prerequisito explícito de multi-usuario; hasta entonces mantener el trust model documentado (fuera de alcance funcional de esta ola salvo pedido).

#### M5 — Bootstrap grueso de `cmd/router` mezclado con handlers

- **Ubicación:** `cmd/router/main.go` `main()` (~176 líneas de wiring).
- **Por qué importa:** Dificulta reutilizar cableado; alarga el archivo de handlers.
- **Dirección:** Tras H2, opcional `wire.go` / `bootstrap.go`. Un split handlers/wire basta.

### Low

#### L1 — CSS embebido y estilos inline residuales

- **Ubicación:** `<style>` en `index.html`; `style='…'` en rail vacío.
- **Dirección:** `web/css/master.css` al descomponer (C1).

#### L2 — `root_states` del API infrautilizado en el cliente

- **Ubicación:** respuesta `SubtopicProgress`; UI usa `subtreeProgress` local.
- **Dirección:** Resolver junto con H1.

#### L3 — Paquetes Go restantes en buena forma de tamaño

- **Ubicación:** `pkg/*` (máx. ~332 `gateway.go`); `internal/testenv` pequeño.
- **Nota:** Contexto positivo — no reorganizar por moda.

---

## Temas especiales (pedidos explícitos)

### 1) Descomposición de `index.html` (vanilla, sin framework)

**Veredicto:** Sí descomponer. El monolito ya cruzó el umbral.

**Límites de módulo propuestos** (bajo `cmd/master-web/web/`, preferible ES modules nativos si el `FileServer` embed los sirve):

| Artefacto | Responsabilidad |
|-----------|-----------------|
| `index.html` | Solo shell: query bar, stage, rail, status, dev details |
| `css/master.css` | CSS actual |
| `js/dom.js` | Mapa `el`, clear-buttons, `setStatus`, escape/sanitize |
| `js/api.js` | `api()`, errores HTTP/JSON |
| `js/session.js` | `studentId`, `loadGeneration` / stale |
| `js/progress.js` | `subtreeProgress`, `deriveProgress`, badges/optimista (**único** dueño JS del algoritmo) |
| `js/hierarchy.js` | `renderHierarchy` / acordeón DOM |
| `js/botonera.js` | legacy / tabs / matriz / `recordBotonera` |
| `js/stage.js` | media, markdown, waiting |
| `js/station.js` | query, `handleMatched`, `pollStation` |
| `js/voice.js` | SpeechRecognition |
| `js/app.js` | wiring + `renderDev` / estado global mínimo |

**Orden de extracción:** (1) CSS, (2) `progress.js` + parity, (3) `session.js`, (4) renderers rail/stage, (5) station/voice. Cada paso: Playwright `progress` + chips en verde.

**No hacer:** introducir bundler/framework “porque el archivo es grande”.

### 2) Estructura de handlers del router

**Estado:** 7 RPCs + wiring en un solo `main.go` (~501). Tamaño aún aceptable; el problema es **duplicación** (H2) y la trayectoria.

**Dirección:**

1. Extraer `Server` + handlers a un paquete importable.
2. Dejar `cmd/router/main.go` como composition root.
3. Reemplazar `inProcessRouter` del gateway test por ese `Server` real.
4. Archivo por dominio (`query.go`, `interactive.go`, `progress.go`) **solo** si el package supera ~600–800 líneas tras la extracción.

**No hacer:** mover lógica de negocio fuera de `pkg/dua` / `pkg/vector` hacia el package de handlers.

### 3) Riesgo de dual ownership del progreso

| Capa | Qué hace hoy | Rol deseable |
|------|----------------|--------------|
| Go `ProgressForTree` | Canónico, testeado: total, opened pre-order, estados de **raíz** | **Dueño del algoritmo de agregación y del contrato API** |
| JS `subtreeProgress` / `deriveProgress` | Espejo + estados por **cualquier** nodo + UI optimista | **Dueño de presentación y optimismo**; algoritmo solo con parity o vía API enriquecida |
| `InteractionStore` | Fuente de verdad de “qué se abrió” (proceso) | Persistencia de hechos (`opened`), no de agregados |

**Riesgos de divergencia concretos:**

- Cambio de semántica de `visited`/`partial`/`unvisited` en un solo lado.
- Tratamiento de IDs abiertos desconocidos / borrados del árbol.
- Orden de `opened_subtopic_ids` (Go garantiza pre-order; el set optimista puede desalinearse con IDs huérfanos).
- El cliente ignora `root_states` del servidor → el campo del contrato puede mentir respecto a lo pintado.

**Propiedad a largo plazo (preferida):**

1. **Hechos:** `opened` en store (servidor).
2. **Agregación:** solo Go (o fixtures generadas desde Go).
3. **UI:** consume agregados del servidor (idealmente con estados por nodo) **o** módulo JS cubierto por los mismos goldens que `subtopic_progress_test.go`.
4. Optimismo: mutar el set local de opened y re-aplicar el módulo parity; reconciliar en la próxima carga.

---

## Buckets de priorización para Ola 4

### Soon (antes o en paralelo temprano de features 4.c / 4.d)

- **C1** descomposición mínima: CSS + `progress.js` + `session.js`.
- **H1** parity fixtures (JSON golden compartido + test Go + assert Playwright).
- **H2** extracción de handlers compartidos (desbloquea auth/RPC nuevos sin triplicar).
- Ítems de producto ya en `add-ola4-live-node-policy`: TTL live index, promoción, observabilidad del margen (bloque 4.c).

### Later

- **H3** encapsular generation tokens al completar el split JS.
- **M1** unificar adapter `LiveGenerator`.
- **M2** mover `InteractionStore` de archivo.
- **M3** partir `playwright-check.mjs`.
- **M5** split wire/bootstrap del router.
- Auth real (**M4**) cuando el alcance deje de ser prototipo single-trust (registrado fuera de esta ola).

### Discard-with-justification (candidatos a no hacer)

| Candidato | Justificación |
|-----------|---------------|
| Framework SPA para Master | Costo alto; rompe embed+prototipo; vanilla modules bastan. |
| Partir `cmd/router` en un archivo por RPC *ahora* | Prematuro; primero un package único (H2). |
| Eliminar el agregado optimista del cliente | Empeora UX; mejor parity o API más rica. |
| Reorganizar `pkg/{vector,dua,rag}` por tamaño | No hay violación 1k (L3). |
| WASM/codegen Go→JS del progreso | Overkill frente a fixtures JSON. |

### Ya planificado en otros bloques (no reinventar aquí)

- **4.c** — TTL de nodos live + `PromoteLiveStation` (`add-ola4-live-node-policy`).
- **4.d** — `calibrate --apply`, normalización de query (typos), piso RAG / simmatrix a chunks.
- **Más adelante** — auth multi-usuario, SQLite perfiles, síntesis LLM A5, captions/transcript A4.

---

## Explicit non-goals

Esta change **no implementa** descomposición, parity, extracción de handlers, adapters ni cambios de contrato. Cualquier implementación debe nacer de tasks posteriores explícitas tras la priorización revisor + Dario.
