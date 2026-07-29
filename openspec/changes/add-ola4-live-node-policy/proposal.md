# Proposal: Deuda Ola 4 — política de nodos live en el índice

## Contexto

Ola 3.b (C1) cerró el prototipo web del nodo interactivo. Durante la validación funcional apareció un efecto de sesión larga: cada miss registra una estación en vivo en el mismo índice k-NN que los nodos curados, así que tras varias dudas novel una consulta real matcheó `live://stations/…` en lugar del nodo interactivo correspondiente.

El parche que entró en Ola 3.b es un **margen**: `vector.Node.IsLiveGenerated` marca las estaciones y `Index.Nearest` solo devuelve una live si supera al mejor curado por más de `LivePreferenceMargin` (0.05). Es suficiente para que lo curado sea el default sin volver inalcanzable una estación ya generada, pero no resuelve el crecimiento del índice. Ola 4.c implementa el ciclo mínimo: expiración de nodos live y promoción docente de una estación revisada.

## Objetivo

Definir el ciclo de vida completo de los nodos generados en vivo: cuánto viven, cuándo se descartan y cómo asciende a curado el material que demostró servir.

## Capabilities

- **live-node-lifecycle:** expiración perezosa de nodos live y promoción manual, idempotente y persistente de estaciones listas a nodos curados.

## Alcance incluido (Ola 4)

1. **TTL de nodos live en el índice** — hoy `StationLedger` tiene TTL (`AVLP_STATION_TTL`, default 24h) pero el nodo indexado no expira: sobrevive a la estación que lo originó. Decidir si el TTL del ledger debe desalojar la entrada k-NN y con qué granularidad (por estudiante, global).
2. **Promoción manual a curado** — una estación que se repite es señal de hueco en el currículum. Falta el camino explícito (revisión humana → `embedding_descriptor` + botonera → nodo curado) y el registro de qué se promovió y por qué.
3. **Persistencia del contenido promovido** — el seed curado conserva el markdown y las fuentes de la estación para seguir siendo útil tras reiniciar el proceso.

## Later

- **Calibración del margen:** derivar 0.05 desde el harness junto con el umbral.
- **Aislamiento por estudiante:** requiere autenticación y autorización; el prototipo aún confía en IDs declarados por cliente.
- **Observabilidad:** contadores de live vivos y decisiones ganadas por el margen.
- **Contenido al re-matchear una live no promovida:** resolver contra ledger o persistir contenido fuera del índice.

## Alternativa descartada en Ola 3.b

**Excluir los nodos live del matching estático.** Más simple y elimina el problema de raíz, pero cada repetición de una duda novel regeneraría la estación (costo de RAG y respuesta distinta ante la misma pregunta), y el estudiante que vuelve pierde el material que ya leyó. Queda como opción si la observabilidad del ítem 5 muestra que el margen no alcanza.

## Fuera de alcance

- Vector DB externa / HNSW productivo (sigue fuera desde Ola 2).
- Cambios al contrato `VeDims=5` vs `ContentEmbedDims=64`.

## Riesgos

- Un TTL agresivo puede borrar la estación mientras el estudiante todavía la está leyendo: el desalojo debe mirar el ledger, no solo la antigüedad.
- La promoción manual sin criterio explícito abre la puerta a currículum no revisado.
