## Context

See proposal.md. El patrón de clientes HTTP OpenAI-compatible ya existe
(`HTTPEmbedder`, `HTTPSynthesizer`). El STT reutiliza el mismo estilo de env,
retry en 5xx y errores claros.

## Goals / Non-Goals

**Goals:** transcribe en gateway, MediaRecorder en cliente, cascada explícita,
accesibilidad intacta, setup documentado para Mac.

**Non-Goals:** empaquetar whisper en el binario AVLP; forzar un único runtime
STT; tests automatizados de micrófono real.

## Decisions

### 1. Endpoint OpenAI `/audio/transcriptions`

Compatible con whisper.cpp server y faster-whisper-server. Multipart campo
`file` + `model` + `language`.

### 2. Cascada en el cliente

1. `stt_enabled` (sesión) → MediaRecorder + `/api/transcribe`
2. else `SpeechRecognition` → Web Speech (actual)
3. else no renderiza micrófono

El modo activo se muestra en el panel de desarrollo.

### 3. Límites

Audio máximo ~10 MiB; grabación corta a 60s. Sin auto-enviar.

### 4. Recomendación Mac

`whisper-server` (whisper.cpp) con modelo `small` o `base` para español: buen
balance tamaño/calidad en Apple Silicon; comando documentado en README.

## Risks / Trade-offs

- **[Risk]** Formatos MIME del MediaRecorder varían por browser → enviar
  filename/extensión coherente.
- **[Trade-off]** Playwright no puede ejercer micrófono real → fallback +
  checklist humana.

## Migration Plan

Sin migración de datos. Default sin `AVLP_STT_URL` = comportamiento previo.
