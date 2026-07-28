# Checklist manual — prototipo Master (PR 6.2)

Verificación humana (Dario / revisor). No hay tests automatizados del DOM.

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
- [ ] Ejemplo «duda novel (live)» → pantalla de espera con **mensaje rogeriano visible** (no spinner mudo) y polling ~2s hasta ready/failed; ready muestra `live_content` en Markdown
- [ ] Panel de desarrollo plegado por defecto; al abrir muestra `student_id` de sesión (sin localStorage) y similitud del último match

## Teclado

- [ ] Tab recorre: skip-link → duda → frustración → Buscar → chips → Stage → opciones de botonera → mutación → panel dev
- [ ] En botonera tipo tabs (depth/cognitive/emergency): flechas ↑/↓ o ←/→ mueven selección; Enter/Espacio activa
- [ ] En acordeón: Enter/Espacio expande/colapsa (`aria-expanded`); se puede abrir un hijo (p. ej. Motor) sin haber activado Asientos
- [ ] Foco siempre visible (anillo azul)

## Lector de pantalla (VoiceOver / NVDA / similar)

- [ ] Stage tiene `aria-live="polite"`: al pasar a espera o al cambiar el clip se anuncia el contenido nuevo sin robar foco de forma agresiva
- [ ] La línea de estado (`role="status"`) anuncia errores y confirmaciones
- [ ] Botonera schema expone `role="tablist"` / `role="tab"` (o botones con nombre accesible en matriz/legacy)
- [ ] Acordeón: botón con `aria-expanded` y panel asociado vía `aria-controls`
- [ ] Textos de UI en español (sin jerga técnica de umbral/threshold en la espera)

## Visual / motion

- [ ] Contraste texto/fondo razonable (AA) en Stage oscuro y botonera clara
- [ ] Con `prefers-reduced-motion: reduce`, la barra de espera no anima (o queda estática)
