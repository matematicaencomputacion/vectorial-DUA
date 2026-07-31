## ADDED Requirements

### Requirement: Voz estudiante sin andamiaje en miss vacío

Cuando no hay chunks RAG, la síntesis SHALL responder en 2–4 frases cálidas
sin mencionar estructuras internas (DUA, fuentes, micro-ejercicio, rogeriano)
ni dejar encabezados terminados en `:` sin contenido. SHALL invitar con temas
curados disponibles cuando existan.

#### Scenario: Contexto vacío

- **GIVEN** retrieval sin hits
- **WHEN** se genera la estación
- **THEN** el contenido no contiene jerga interna ni cola `…:` y no emite
  secciones Fuentes/Micro-ejercicio vacías
