## Why

El contrato de medios DUA asume que todo el mundo ve y escucha: no hay
subtítulos, transcripciones, audiodescripción ni texto alternativo. La arista
A4 del informe original es la última pendiente de alineación pedagógica.

## What Changes

- Agrega `captions_url`, `transcript`, `alt_text` y `audio_description_url` a
  variantes de botonera, subtemas y al nodo raíz (`stage_media_default`).
- Refleja los campos en protobuf + `convert.go`.
- Validación gradual: `Validate()` no exige los campos; `AccessibilityReport`
  detecta huecos; `LoadDir` los loguea; `AVLP_REQUIRE_MEDIA_A11Y=true` los
  convierte en error.
- Completa los 6 seeds curados con `alt_text` y `transcript` reales.
- `PromoteLiveStation` copia el contenido live a `transcript`.
- Stage: alt text, toggle de transcripción y `<track>` de captions cuando hay
  `<video>`.

### Incluido

- Tests de contrato/`AccessibilityReport`, Playwright del toggle, README.

### Fuera de alcance

- CDN real de VTT/audiodescripción, seed promovido local del usuario.
- Auth, STT local (bloques 5.c / 5.d).

### Rollback

Campos omitidos son compatibles; quitar el flag restaura el modo gradual.
Sin migración de datos obligatoria.

## Capabilities

### New Capabilities

- `media-accessibility`: Alternativas textuales y metadatos a11y en el contrato
  de medios DUA, reporte gradual y UI de Stage.

### Modified Capabilities

Ninguna.

## Impact

`pkg/dua`, `proto/interactive_node.proto`, seeds interactivos, `cmd/master-web`
(Stage/rail), README y tests.
