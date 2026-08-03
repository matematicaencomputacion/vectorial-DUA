# Evidencia Playwright — Ola 3.b / C1

Generada con `node cmd/master-web/verify/playwright-check.mjs`. Por defecto el
script levanta un stack hermético: copia los seeds
`data/nodes/interactive/*.json` **trackeados en git** a un dir temporal y
arranca router + master-web con `AVLP_INTERACTIVE_NODES_DIR` ahí (puertos
efímeros), `AVLP_SIMILARITY_THRESHOLD=0.482` (calibrate hash 2026-08-03) y
`AVLP_CONFIG_PATH` aislado del `data/avlp.json` local.

| Archivo | Qué demuestra |
|---------|----------------|
| `flow-01-inicial.png` | Página fresca: `#ask-box` oculto, HTML con `Cache-Control: no-cache` |
| `flow-02-boton-limpieza.png` | ✕ visible con texto, vacía el campo y restaura el foco |
| `chip-01-static.png` … `chip-07-live.png` | Cada chip llega al nodo que anuncia su etiqueta |
| `flow-03-botonera-depth.png` | Botonera depth con copy neutral («Resumen express») |
| `flow-04-mutate-live.png` | Botón LIVE en la botonera, label sin archivo interno |
| `flow-05-live-honesto.png` | Miss path sin hits RAG: «No encontré material verificado…» |
| `flow-06-router-caido.png` | Router caído: mensaje contenedor, sin `dial tcp` |
| `orientation-01-ubicarte.png` | Rail «Para ubicarte» tras async/await (`AVLP_ONLY=orientation`) |

Mensaje esperado con router caído: *«No pudimos conectar con el tutor en este momento; probá de nuevo en un instante»*.

## Ola 3.c / C4 — progreso de subtemas

Generada con `AVLP_ONLY=progress node cmd/master-web/verify/playwright-check.mjs`.
El routing al seed automóvil se fija en Playwright para aislar esta prueba; el
nodo, el GET de progreso y `RecordSubtopicInteraction` usan el stack real.

- `progress-01-clean.png`: acordeón limpio, contador 0 de 5 y estados «○ Por explorar».
- `progress-02-motor-visited.png`: Motor «✓ Visitado», Caja Central «◐ Exploración iniciada» y contador 1 de 5, actualizado sin otro GET de progreso.
- `progress-03-reconciled.png`: nueva carga en la misma pestaña; el `student_id` de `sessionStorage` recupera Motor desde el router.
- `progress-04-stale-race.png`: búsqueda A lenta + B inmediata; solo B queda en el rail (invalidación por generación).
