## Why

La Web Speech API manda el audio a servidores de Google; en la validación en
vivo produjo errores de red recurrentes. Cerrar esa dependencia externa con un
STT local OpenAI-compatible es el último bloque de Ola 5 antes del tag.

## What Changes

- Cliente STT HTTP (`pkg/stt`) hacia `POST …/audio/transcriptions`.
- Gateway: `POST /api/transcribe` (multipart) + exposición de `stt_enabled` en
  la sesión.
- `voice.js`: MediaRecorder → STT local; cascada progresiva
  (local → Web Speech → sin botón); tope 60s; panel de desarrollo con modo.
- README (setup Mac) + MANUAL_CHECKLIST + Playwright del fallback.

### Incluido

- Tests con `httptest` (sin backends reales).
- Config `AVLP_STT_*`.

### Fuera de alcance

- UI de configuración de STT; modelos embebidos en el binario; diarización.

### Rollback

Vaciar `AVLP_STT_URL` restaura la cascada Web Speech / sin botón.

## Capabilities

### New Capabilities

- `local-voice`: dictado vía STT local OpenAI-compatible con cascada progresiva.

### Modified Capabilities

Ninguna.

## Impact

`pkg/stt`, `pkg/webgateway`, `cmd/master-web` (voice.js, session, UI, README),
tests y verify Playwright.
