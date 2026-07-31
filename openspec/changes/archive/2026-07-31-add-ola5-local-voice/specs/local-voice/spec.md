## ADDED Requirements

### Requirement: Transcripción vía STT local

El gateway SHALL exponer `POST /api/transcribe` que reenvía audio multipart a un
backend OpenAI-compatible (`…/audio/transcriptions`) cuando `AVLP_STT_URL` está
configurado. Sin URL, SHALL indicar que el STT no está disponible.

#### Scenario: Transcripción exitosa

- **GIVEN** un STT mock que responde `{"text":"hola"}`
- **WHEN** el cliente envía audio multipart
- **THEN** recibe el texto transcrito

#### Scenario: Retry en 5xx

- **GIVEN** un STT que falla una vez con 503 y luego responde OK
- **WHEN** se transcribe
- **THEN** el cliente STT reintenta y completa

### Requirement: Cascada progresiva de dictado

La UI SHALL preferir STT local si está habilitado; si no, Web Speech API; si
ninguna opción, no renderizar el micrófono. El modo activo SHALL verse en el
panel de desarrollo.

#### Scenario: Sin STT ni SpeechRecognition

- **GIVEN** `stt_enabled=false` y sin `SpeechRecognition`
- **WHEN** carga la página
- **THEN** no hay botón de micrófono

### Requirement: Salvaguarda de grabación

La grabación local SHALL cortarse sola a los 60 segundos y no auto-enviar el
formulario de duda.
