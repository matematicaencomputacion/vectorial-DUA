# ADR-001 — Criterio de lenguajes: C++ vs Go en el ecosistema AVLP

**Estado:** Aceptado · **Fecha:** 2026-08-03 · **Autores:** Dario Bublitz (criterios), revisión de arquitectura (análisis y umbrales)

**Alcance:** AVLP (vectorial-DUA) y los proyectos que converjan con él (plataforma de ejercicios, carriles inglés/Python/matemática).

---

## 1. Contexto

El proyecto adopta una estrategia de largo plazo orientada a eficiencia extrema de hardware (CPU/RAM) y velocidad de desarrollo, con una rúbrica estricta para decidir lenguaje por componente. Este documento fija esa rúbrica, la aplica al ecosistema **real y medido** de AVLP, y define los umbrales objetivos que dispararían una reevaluación. La regla de oro que gobierna todo el documento: **la decisión se toma con números medidos, nunca con intuiciones.**

---

## 2. La rúbrica

### 2.1 Cuándo usar C++ OBLIGATORIAMENTE (el núcleo de alto rendimiento)

- Procesamiento numérico intensivo, álgebra lineal, motores de cálculo vectorial y manipulación directa de buffers de memoria.
- Sistemas donde la latencia debe ser determinista (evitando pausas por Garbage Collection).
- Componentes orientados a hardware, procesamiento gráfico de bajo nivel (pipelines de renderizado, shaders) y algoritmos críticos donde cada ciclo de CPU cuenta.
- **Restricción obligatoria:** gestión de memoria moderna (RAII, `std::unique_ptr`/`std::shared_ptr`), análisis estático estricto y sanitizers (ASan/UBSan/TSan) para evitar fugas y undefined behavior.

### 2.2 Cuándo usar Go EXCLUSIVAMENTE (infraestructura, redes y orquestación)

- Microservicios, pasarelas de API (routers gRPC/HTTP), servidores web y sistemas concurrentes I/O-bound.
- Herramientas CLI de soporte, automatizaciones y scripts de integración o despliegue.
- Casos donde la velocidad de entrega y la simplicidad de la concurrencia (goroutines) superan la necesidad de optimización extrema de CPU.

### 2.3 Cuándo NO conviene C++ (antipatrones a rechazar)

- Interfaces web, lógica de negocio de iteración rápida y capas de presentación, donde el costo de desarrollo (boilerplate, complejidad de compilación) frena la agilidad de producto.
- Servicios de red puros donde Go ofrece la misma resiliencia concurrente con una fracción del esfuerzo.

### 2.4 Formato requerido ante toda propuesta de diseño

Cada componente nuevo o refactorización debe justificar el lenguaje elegido por: **perfil de carga** (CPU-bound vs I/O-bound), **huella de RAM**, y **mantenibilidad a largo plazo**.

---

## 3. Aplicación al ecosistema AVLP (datos medidos, no intuiciones)

### 3.1 Números de referencia del sistema real

| Métrica | Valor medido |
|---|---|
| Dimensionalidad de embeddings (bge-m3) | 1024 |
| Nodos en el índice de ruteo | ~17 (12 seeds + 5 demo) |
| Chunks en la base de conocimiento RAG | 12 |
| Costo de una consulta `Nearest` (fuerza bruta, Go escalar) | ~17.000 mult. ≈ **20–50 µs** |
| Llamada de embedding a Ollama (misma consulta) | **20–100 ms** |
| Síntesis LLM de una estación (qwen3:4b) | **10–30 s** |
| RAM por nodo (1024 × float32) | 4 KB (10.000 nodos = 40 MB) |
| Pausas del GC de Go con ese heap | **< 1 ms** |

**Lectura:** el cómputo propio del router es el **0,05%** del presupuesto de latencia de una consulta. Reescribir el kernel vectorial en C++ hoy optimizaría ese 0,05% y pagaría el peaje de frontera de cgo (~1–2 µs por llamada, pérdida de `-race`, sanitizers y toolchain nativo en un CI que es un runner Ubuntu pelado). El requisito de "latencia determinista sin GC" no muerde: con heaps de decenas de MB, las pausas de Go quedan tres órdenes de magnitud por debajo de la latencia de una sola llamada al embedder.

### 3.2 Veredicto por componente

| Componente | Perfil de carga | RAM | Veredicto | Justificación |
|---|---|---|---|---|
| `pkg/vector` (Index.Nearest, cosine) | CPU-bound numérico | KB–MB | **Go hoy; reevaluar al escalar (§4)** | 20–50 µs por consulta vs 20–100 ms de red; cgo costaría más que lo que ahorra |
| `pkg/rag` (Retrieve, chunker, HashEmbedder) | CPU-bound trivial | KB | **Go** | FNV + sort sobre 12 chunks; ruido |
| `internal/routerserver` + gRPC | I/O-bound | MB | **Go exclusivo** | Rúbrica §2.2 textual: pasarela de API |
| `pkg/webgateway`, `pkg/session`, `pkg/stt` | I/O-bound, red pura | MB | **Go exclusivo** | Rúbrica §2.3 rechaza C++ acá |
| `cmd/*` (harness, graph-sync, router-client) | Tooling CLI | — | **Go exclusivo** | Rúbrica §2.2 |
| Frontend Master (`cmd/master-web/web/`) | Presentación | — | **JS vanilla** | Rúbrica §2.3; nadie propone C++ en el browser |
| Inferencia LLM + embeddings + STT | **CPU/GPU-bound extremo** | GB | **Ya es C++ (delegado)** | Ver §3.3 — acá vive el 90% real de los ciclos |
| Neo4j (grafo de conocimiento, Ola 7) | Store externo | — | **JVM de terceros** | No es código propio; read-through con caché y fallback a archivo |

### 3.3 El hallazgo central: la estrategia ya está implementada

El 90% real de los ciclos de CPU que quema este sistema **no está en el router — está en la inferencia** de bge-m3 (embeddings) y qwen3 (síntesis). Y eso **ya corre en C++**: Ollama es un envoltorio de **llama.cpp/ggml** (SIMD, cuantización, gestión de memoria manual); Whisper, cuando se active, es **whisper.cpp**.

La arquitectura vigente es exactamente "C++ donde viven los ciclos, Go donde vive el cableado", con una diferencia deliberada respecto de cgo: la frontera es **de proceso (HTTP/loopback), no de linkeo**. Esa forma es superior para este ecosistema porque:

1. Conserva `-race`, builds de segundos y un CI sin toolchain nativo.
2. Aísla el undefined behavior del lado de quien lo mantiene (ggml tiene más ingenieros mirando sus buffers que este equipo).
3. Permite cambiar de motor (Ollama → llama-server → vLLM) sin recompilar nada propio.

**Corolario:** "usar C++ obligatoriamente" en este ecosistema se instancia, por defecto, como **adoptar C++ escrito, fuzzeado y mantenido por terceros detrás de una frontera de proceso** — no como escribir álgebra lineal artesanal.

---

## 4. Umbrales de disparo (cuándo se reevalúa, con números)

La reevaluación del kernel vectorial se dispara cuando **cualquiera** de estas condiciones se mida (no se estime):

| Disparador | Umbral | Acción |
|---|---|---|
| Latencia del kernel | `Nearest` o `Retrieve` > **5 ms** por consulta en p99, o > **10%** del p99 total | Evaluar ANN (ver abajo) |
| Escala del índice | > **50.000 nodos** × 1024 dims | Evaluar ANN |
| RAM del índice | > **2 GB** de embeddings residentes | Evaluar cuantización (int8/PQ) |
| Requisito nuevo real | Pipeline de renderizado propio, juez de ejercicios con requisitos duros de aislamiento/latencia | Evaluar C++ propio con §5 |

**Cuando el disparador de escala llegue, la respuesta correcta no es el mismo algoritmo más rápido: es otro algoritmo.** Fuerza bruta O(n) se reemplaza por ANN/HNSW O(log n) — y las implementaciones maduras de ANN **ya son C++** (FAISS, hnswlib, el motor de Qdrant). La instanciación preferida sigue el patrón de backends opcionales del repo (quinta repetición): interfaz en el consumidor + `AVLP_ANN_URL` para un motor externo, o una lib cgo **aislada en su propio módulo** si se necesita in-process.

**Instrumentación que sostiene estos umbrales (instrumentado — ver `-suite bench`):**
`go run ./cmd/harness -suite bench` — benchmarks de `Nearest` y `Retrieve` a
100 / 1.000 / 10.000 / 100.000 nodos sintéticos. CI corre los escenarios ≤1K y
falla ante cruce algorítmico; la ladder completa es manual (ver
`harness/README.md`). Cola histórica: [docs/ola7-backlog.md](../ola7-backlog.md).

---

## 5. Reglas obligatorias si algún día se escribe C++ propio

Además de las restricciones de la rúbrica (§2.1), todo módulo C++ de este ecosistema cumple:

1. **Frontera con ABI C estable** (`extern "C"`); sin excepciones cruzando el límite; errores por código de retorno.
2. **RAII y smart pointers** en todo el interior; prohibido `new`/`delete` desnudo; `std::span`/`string_view` en las firmas de entrada.
3. **CI dedicado** con ASan + UBSan + TSan en job propio (no contamina el CI de Go); build con `-Wall -Wextra -Werror`.
4. **Fuzzing** del parser de toda entrada externa (libFuzzer) antes del primer release.
5. **Repo o módulo separado**: el build de Go nunca depende de un compilador nativo; el módulo C++ publica artefactos versionados.
6. **Benchmark de frontera**: se mide y documenta el costo del cruce (cgo o IPC) junto al beneficio neto; si el beneficio neto medido es < 2×, el módulo no entra.

---

## 6. Decisión

1. **Hoy (v0.5.x, Ola 7):** cero componentes propios en C++. Go en toda la infraestructura, orquestación y tooling; JS vanilla en presentación; el núcleo numérico real (inferencia) delegado a C++ de terceros (llama.cpp/ggml, whisper.cpp) detrás de fronteras de proceso.
2. **Umbral cruzado (medido por la suite bench):** ANN en C++ de terceros detrás del patrón de backend opcional. C++ propio solo si aparece un requisito de §4 fila 4, bajo las reglas de §5.
3. **Este documento se revisa** cuando cambie la escala del índice en un orden de magnitud, cuando converja el proyecto de ejercicios, o cuando un benchmark de la suite cruce un umbral — lo que ocurra primero.

---

*Referencias internas: patrón de backends opcionales (`rag.Embedder`/`HTTPEmbedder`, `rogerian.Synthesizer`, `stt.Transcriber`, `dua.ProfileRepository`, `knowledge.KnowledgeGraph`); calibración con evidencia (suites `calibrate`/`simmatrix`/`bench`); CI hermético (`scripts/test-clean.sh`).*
