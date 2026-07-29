# Evidencia Playwright — Ola 3.b / C1

Generada con `node cmd/master-web/verify/playwright-check.mjs` contra `master-web`
conectado al router (flujo completo) y, en modo `AVLP_ONLY=routerdown`, sin router.

| Archivo | Qué demuestra |
|---------|----------------|
| `flow-01-inicial.png` | Página fresca: `#ask-box` oculto, HTML con `Cache-Control: no-cache` |
| `flow-02-boton-limpieza.png` | ✕ visible con texto, vacía el campo y restaura el foco |
| `chip-01-static.png` … `chip-07-live.png` | Cada chip llega al nodo que anuncia su etiqueta |
| `flow-03-botonera-depth.png` | Botonera depth con copy neutral («Resumen express») |
| `flow-04-mutate-live.png` | Botón LIVE en la botonera, label sin archivo interno |
| `flow-05-live-honesto.png` | Miss path sin hits RAG: «No encontré material verificado…» |
| `flow-06-router-caido.png` | Router caído: mensaje contenedor, sin `dial tcp` |

Mensaje esperado con router caído: *«No pudimos conectar con el tutor en este momento; probá de nuevo en un instante»*.
