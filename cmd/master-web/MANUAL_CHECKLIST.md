# Checklist manual — prototipo Master (Ola 3.c)

Verificación humana (Dario / revisor), complementaria a Playwright.

## Arranque

```bash
go run ./cmd/router
go run ./cmd/master-web
# abrir http://127.0.0.1:8080
```

## Flujo funcional

- [ ] Duda con ejemplo «variables y scope» → carga nodo interactivo (Stage + botonera depth)
- [ ] Toque de opción de profundidad actualiza el Stage (título / media_url) y el panel de desarrollo refleja cambio en $V_e$
- [ ] Ejemplo «async/await» → botonera cognitiva; «PostGIS» → matriz combined; «automóvil» → acordeón (abrir Motor sin pasar por Asientos)
- [ ] «+ Tengo una duda diferente» → aparece botón LIVE en la botonera
- [ ] Ejemplo «fuera de tema (honesto)» → estación **ready** con contenido no vacío
  y **sin** fuentes espurias (p. ej. no citar `env-variables.md` en una duda de
  física de partículas). Playwright solo verifica esa estructura estable; **el
  juicio sobre la calidad de la redacción generativa es humano** (no
  automatizable con substring). Con `AVLP_LLM_URL` vacío el copy extractivo sí
  es determinista («No encontré material verificado…») y sirve de referencia.
- [ ] Nodo estático / estación live: **no** muestra «+ Tengo una duda diferente»; sí el hint «Para una duda nueva…»
- [ ] Error de API (p. ej. mutación inválida) aparece en la franja de estado con `aria-live="assertive"`
- [ ] Label «Probá con:» + chips «Ejemplo: …»; al tocar un chip, el status indica que hay que pulsar «Buscar estación»

## Progreso de subtemas (Compromiso / autorregulación)

- [ ] Primera carga de «automóvil»: contador accesible «Exploraste 0 de 5 subtemas»; cada rama muestra símbolo + texto («○ Por explorar»), nunca solo color
- [ ] Expandir Caja Central → Motor y pulsar «Abrir en Stage: Motor»: Motor pasa a «✓ Visitado», Caja Central a «◐ Exploración iniciada» y el contador a «Exploraste 1 de 5 subtemas»
- [ ] El cambio aparece inmediatamente al tocar, sin un segundo GET de progreso; al volver a cargar el nodo, el estado se reconcilia con el router
- [ ] El copy invita sin presionar («Te queda por explorar: …»): no hay porcentajes, barras, rachas ni llamados a completar 100%
- [ ] Reiniciar **solo `master-web`**, conservar el router y recargar la misma pestaña: `student_id` se mantiene en `sessionStorage` y Motor sigue visitado
- [ ] Reiniciar el router sí borra este progreso: `InteractionStore` continúa siendo en memoria en Ola 3.c
- [ ] Panel de desarrollo: muestra por separado el payload crudo de progreso devuelto por la API y el estado local optimista

## Voz (cascada: STT local → Web Speech → sin botón)

Playwright **no** puede ejercer un micrófono real. Los casos de captura/permiso
siguen siendo checklist humana. Automatizado: fallback sin SpeechRecognition ni
STT → no hay `.mic-btn`; `stt_enabled` en sesión; panel `voice: mode=…`.

### Humano — STT local (`AVLP_STT_URL`)

- [ ] Con STT arriba: micrófono aparece; panel `voice: mode=local`; grabar →
  «Grabando» (símbolo + texto) → segundo toque → texto en el textarea **sin**
  auto-enviar; corte automático a ~60 s
- [ ] Denegar permiso de micrófono → mensaje amable (`aria-live` assertive)
- [ ] Ctrl+M inicia/detiene en «Tu duda»
- [ ] Con nodo interactivo: mismo flujo en «+ Tengo una duda diferente»

### Humano — Web Speech (sin `AVLP_STT_URL`, Chrome/Edge)

Soportado donde exista `SpeechRecognition` / `webkitSpeechRecognition`. Si no
hay API ni STT, el botón **no aparece**.

- [ ] Micrófono junto a «Tu duda»: al tocar (o **Ctrl+M**) pasa a «Grabando»;
  el texto aparece; al cerrar la frase se detiene **sin** auto-enviar
- [ ] Segundo toque cancela a mitad de dictado; estado visible
- [ ] Error de red de Google → mensaje en español que menciona la nube / STT local
- [ ] Denegar permiso → mensaje amable, nunca silencio

- [ ] Página fresca: el bloque «+ Tengo una duda diferente» **no** es visible (evidencia Playwright: `verify/out/01-fresh-ask-box-hidden.png` / flow-01)
- [ ] Con router caído: el estado muestra mensaje contenedor en español, **sin** `dial tcp` / transport (evidencia: `verify/out/02-router-down-friendly.png`)
- [ ] Sin STT ni SpeechRecognition: no hay `.mic-btn` (evidencia: `flow-09-voice-fallback-none.png`)

## Teclado

- [ ] Tab recorre: skip-link → duda → micrófono (si hay) → frustración → Buscar → chips → Stage → opciones de botonera → mutación → panel dev
- [ ] Ctrl+M inicia/cancela dictado en «Tu duda» (Chrome/Edge)
- [ ] En botonera tipo tabs (depth/cognitive/emergency): flechas ↑/↓ o ←/→ mueven selección; Enter/Espacio activa
- [ ] En acordeón: Enter/Espacio expande/colapsa (`aria-expanded`); se puede abrir un hijo (p. ej. Motor) sin haber activado Asientos
- [ ] Tras abrir Motor, el foco permanece operable y el acordeón actualizado sigue recorrible por Tab
- [ ] Foco siempre visible (anillo azul)

## Lector de pantalla (VoiceOver / NVDA / similar)

- [ ] Stage tiene `aria-live="polite"`: al pasar a espera o al cambiar el clip se anuncia el contenido nuevo sin robar foco de forma agresiva
- [ ] La línea de estado (`role="status"`) anuncia errores y confirmaciones
- [ ] Botonera schema expone `role="tablist"` / `role="tab"` (o botones con nombre accesible en matriz/legacy)
- [ ] Acordeón: botón con `aria-expanded` y panel asociado vía `aria-controls`
- [ ] El lector anuncia «Exploraste N de M subtemas» al cambiar y cada botón incluye «Visitado», «Exploración iniciada» o «Por explorar» en su nombre accesible
- [ ] Textos de UI en español (sin jerga técnica de umbral/threshold en la espera)

## Visual / motion

- [ ] Contraste texto/fondo razonable (AA) en Stage oscuro y botonera clara
- [ ] Con `prefers-reduced-motion: reduce`, la barra de espera no anima (o queda estática)
