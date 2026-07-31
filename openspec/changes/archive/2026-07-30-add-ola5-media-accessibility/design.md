## Context

See proposal.md — Why. Los seeds y variantes ya cargan `media_url` / formatos
video y audio sin alternativa textual. `Validate()` es estricto en estructura
pero no puede exigir a11y sin romper curriculum y promovidos.

## Goals / Non-Goals

**Goals:**

- Contrato tipado + proto para a11y opcional.
- Reporte y logging sin ruptura; flag de instituto para exigir.
- Seeds curados con transcript/alt reales; promoción reutiliza el markdown.
- Stage usable con teclado y lectores de pantalla.

**Non-Goals:**

- Generar VTT automáticamente; forzar `<video>` bytes reales del CDN de ejemplo.

## Decisions

### 1. Campos planos en cada variante (no wrapper JSON anidado)

Misma forma en Go/JSON/proto para Depth/Cognitive/Emergency/Combined/Subtopic
y nodo raíz. Facilita seeds y convert.

### 2. Gap = medio AV sin alternativa textual

Un ítem con `format_type` video/audio_debate (o URL claramente AV) y `media_url`
requiere `transcript` o `captions_url`. Falta de `alt_text` también se reporta.
`text_hint` / solo `cell_code` no exigen alternativa AV.

### 3. `Validate()` intacto; `AccessibilityReport` + env flag

`AVLP_REQUIRE_MEDIA_A11Y=true` hace fallar `LoadDir`/`Put` vía chequeo post-Validate.
Default: loguear con `Registry.Logf`.

### 4. Promoción → `transcript`

El markdown de la estación es la alternativa textual natural del nodo curado;
también se rellena `alt_text` breve si falta.

## Risks / Trade-offs

- **[Risk]** Seeds incompletos en forks → **Mitigation:** solo log por default.
- **[Risk]** Video placeholder sin bytes → **Mitigation:** DOM con `<video>`+`<track>`
  cuando hay URL de video; el prototipo ya documenta CDN de ejemplo.

## Migration Plan

Desplegar binario + seeds. Rollback: binario anterior ignora campos nuevos en JSON.
