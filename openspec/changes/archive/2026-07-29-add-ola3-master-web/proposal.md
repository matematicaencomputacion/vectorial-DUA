# Proposal: Prototipo web Master — Stage + botonera (Ola 3.b / C1)

## Problema

El layout 70/30 (Stage + botonera) vive en ASCII del README; el contrato interactivo nunca se ejercitó contra una UI real.

## Alcance

### PR 6.1 — ✅ gateway HTTP/JSON

- `cmd/master-web` + `pkg/webgateway`: JSON sobre RPCs del router (`AVLP_WEB_ADDR`, `AVLP_ROUTER_ADDR`)
- Estáticos embebidos (`web/`)
- Tests con router gRPC in-process (bufconn)
- `index.html` servido con `Cache-Control: no-cache` (un cliente con caché viejo no se queda con una UI rota)

### PR 6.2 — ✅ frontend vanilla + cierre C1

- `cmd/master-web/web/index.html` (HTML+CSS+JS, sin build)
- Tres modos: `botonera_schema`, legacy, `hierarchy`
- Flujo query → nodo | pending+poll → ready; mutate; Record*
- A11y DUA (teclado, ARIA, contraste AA, `prefers-reduced-motion`)
- Dictado por voz progresivo (Web Speech API, `es-AR`, Ctrl+M)
- Botón de limpieza (✕) en los campos de duda
- Chips de ejemplo alineados con el nodo que anuncian
- Copy student-facing: label LIVE sin archivo interno; «Resumen express» en depth
- k-NN: nodos live marcados `IsLiveGenerated`; margen 0.05 a favor de curados
- Panel de desarrollo plegado; `student_id` en memoria
- README «Prototipo web» + `MANUAL_CHECKLIST.md` + evidencia Playwright

## Estado

**Cerrada** — tag anotado `v0.3.1-ola3b`. Deuda restante en `add-ola4-live-node-policy` (TTL de nodos live, promoción a curado, contenido al re-matchear).
