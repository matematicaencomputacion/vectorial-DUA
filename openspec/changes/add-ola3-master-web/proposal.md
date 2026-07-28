# Proposal: Prototipo web Master — Stage + botonera (Ola 3.b / C1)

## Problema

El layout 70/30 (Stage + botonera) vive en ASCII del README; el contrato interactivo nunca se ejercitó contra una UI real.

## Alcance

### PR 6.1 — gateway HTTP/JSON

- `cmd/master-web` + `pkg/webgateway`: JSON sobre RPCs del router (`AVLP_WEB_ADDR`, `AVLP_ROUTER_ADDR`)
- Estáticos embebidos (`web/`) con placeholder hasta 6.2
- Tests con router gRPC in-process (bufconn)

### PR 6.2 — frontend vanilla (fuera de este PR)

`web/index.html` fiel al contrato + a11y DUA + checklist manual.
