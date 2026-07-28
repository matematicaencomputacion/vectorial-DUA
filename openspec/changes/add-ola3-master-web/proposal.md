# Proposal: Prototipo web Master — Stage + botonera (Ola 3.b / C1)

## Problema

El layout 70/30 (Stage + botonera) vive en ASCII del README; el contrato interactivo nunca se ejercitó contra una UI real.

## Alcance

### PR 6.1 — ✅ gateway HTTP/JSON

- `cmd/master-web` + `pkg/webgateway`: JSON sobre RPCs del router (`AVLP_WEB_ADDR`, `AVLP_ROUTER_ADDR`)
- Estáticos embebidos (`web/`)
- Tests con router gRPC in-process (bufconn)

### PR 6.2 — frontend vanilla

- `cmd/master-web/web/index.html` (HTML+CSS+JS, sin build)
- Tres modos: `botonera_schema`, legacy, `hierarchy`
- Flujo query → nodo | pending+poll → ready; mutate; Record*
- A11y DUA (teclado, ARIA, contraste AA, `prefers-reduced-motion`)
- Panel de desarrollo plegado; `student_id` en memoria
- README «Prototipo web» + `MANUAL_CHECKLIST.md`
