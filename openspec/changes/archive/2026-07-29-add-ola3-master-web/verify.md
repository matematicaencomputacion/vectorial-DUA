# Verify: Ola 3.b — prototipo web Master (C1)

## Checklist

- [x] Gateway HTTP/JSON sobre el router gRPC (`pkg/webgateway`, `cmd/master-web`)
- [x] UI Stage + botonera: schemas depth/cognitive/emergency/combined, legacy y hierarchy
- [x] Flujo live con polling rogeriano; mutate agrega botón LIVE sin recargar
- [x] Dictado por voz `es-AR` y botón ✕ en los campos de duda
- [x] `Cache-Control: no-cache` en `index.html` (`curl -I`)
- [x] Chips de ejemplo → nodo anunciado (Playwright, captura por chip)
- [x] Copy student-facing: sin archivo en label LIVE; «Resumen express (30–45s)»
- [x] Curados priorizados sobre live en k-NN (`LivePreferenceMargin = 0.05`)
- [x] `go build ./...` + `go test -race ./...` verde en merge a `main`
- [x] Tag anotado `v0.3.1-ola3b`
