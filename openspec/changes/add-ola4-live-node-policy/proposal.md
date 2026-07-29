# Proposal: Deuda Ola 4 — política de nodos live en el índice

## Contexto

Ola 3.b (C1) cerró el prototipo web del nodo interactivo. Durante la validación funcional apareció un efecto de sesión larga: cada miss registra una estación en vivo en el mismo índice k-NN que los nodos curados, así que tras varias dudas novel una consulta real matcheó `live://stations/…` en lugar del nodo interactivo correspondiente.

El parche que entró en Ola 3.b es un **margen**: `vector.Node.IsLiveGenerated` marca las estaciones y `Index.Nearest` solo devuelve una live si supera al mejor curado por más de `LivePreferenceMargin` (0.05). Es suficiente para que lo curado sea el default sin volver inalcanzable una estación ya generada, pero no resuelve el crecimiento del índice. Esta change **no implementa** nada: registra la deuda.

## Objetivo

Definir el ciclo de vida completo de los nodos generados en vivo: cuánto viven, cuándo se descartan y cómo asciende a curado el material que demostró servir.

## Alcance incluido (Ola 4)

1. **TTL de nodos live en el índice** — hoy `StationLedger` tiene TTL (`AVLP_STATION_TTL`, default 24h) pero el nodo indexado no expira: sobrevive a la estación que lo originó. Decidir si el TTL del ledger debe desalojar la entrada k-NN y con qué granularidad (por estudiante, global).
2. **Promoción manual a curado** — una estación que se repite es señal de hueco en el currículum. Falta el camino explícito (revisión humana → `embedding_descriptor` + botonera → nodo curado) y el registro de qué se promovió y por qué.
3. **Calibración del margen** — 0.05 salió del comportamiento observado con bge-m3 y umbral 0.55, no de un barrido. Al crecer el corpus conviene derivarlo del harness (`-suite calibrate`) junto con el umbral.
4. **Aislamiento por estudiante** — hoy la estación de un estudiante es visible en el matching de cualquier otro. Definir si el índice live debe particionarse por `student_id` o quedar compartido a propósito. Relacionado: el prototipo confía en el `student_id` declarado por el cliente para `Record*` y `GetSubtopicProgress` (sin autenticación); una fase multi-usuario requiere autenticación y autorización por estudiante.
5. **Observabilidad** — contador de nodos live vivos y de veces que el margen evitó un desplazamiento, para saber si el parche alcanza o hay que ir a exclusión total.
6. **Contenido al re-matchear una estación** — apareció verificando Ola 3.b: el nodo live queda indexado pero su contenido vive en el `StationLedger`, indexado por `tracking_ulid`. Cuando un estudiante repite la duda y matchea su propio nodo, el Stage no tiene qué mostrar (solo la `resource_url`). Sin esto, «volver a tu estación» no se sostiene y el ítem 1 (TTL) es discutible: hay que decidir si el contenido se guarda con el nodo o si el match resuelve contra el ledger.

## Alternativa descartada en Ola 3.b

**Excluir los nodos live del matching estático.** Más simple y elimina el problema de raíz, pero cada repetición de una duda novel regeneraría la estación (costo de RAG y respuesta distinta ante la misma pregunta), y el estudiante que vuelve pierde el material que ya leyó. Queda como opción si la observabilidad del ítem 5 muestra que el margen no alcanza.

## Fuera de alcance

- Vector DB externa / HNSW productivo (sigue fuera desde Ola 2).
- Cambios al contrato `VeDims=5` vs `ContentEmbedDims=64`.

## Riesgos

- Un TTL agresivo puede borrar la estación mientras el estudiante todavía la está leyendo: el desalojo debe mirar el ledger, no solo la antigüedad.
- La promoción manual sin criterio explícito abre la puerta a currículum no revisado.
