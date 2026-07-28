# Evidencia Playwright — PR 6.2 (hidden + errores amables)

Generada con `node cmd/master-web/verify/playwright-check.mjs` contra `master-web` **sin** router en `:50051`.

| Archivo | Qué demuestra |
|---------|----------------|
| `01-fresh-ask-box-hidden.png` | Página fresca: `#ask-box` no visible (CSS `[hidden]{display:none!important}`) |
| `02-router-down-friendly.png` | Búsqueda con router caído: estado con mensaje contenedor, sin `dial tcp` |

Mensaje esperado: *«No pudimos conectar con el tutor en este momento; probá de nuevo en un instante»*.
