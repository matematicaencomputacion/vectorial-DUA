## ADDED Requirements

### Requirement: Contrato de medios con alternativas opcionales

Las variantes de botonera, los subtemas y el nodo raíz SHALL aceptar
`captions_url`, `transcript`, `alt_text` y `audio_description_url` opcionales,
expuestos también por protobuf.

#### Scenario: Seed sin campos a11y

- **GIVEN** un seed interactivo válido sin campos de accesibilidad
- **WHEN** se valida con `Validate()`
- **THEN** la validación estructural sigue pasando

#### Scenario: Round-trip protobuf

- **GIVEN** un nodo con transcript y alt_text en el dominio
- **WHEN** se convierte a protobuf y se consulta
- **THEN** los campos a11y están presentes en el mensaje

### Requirement: Reporte gradual de huecos de accesibilidad

El sistema SHALL ofrecer `AccessibilityReport` que liste medios de video/audio
sin alternativa textual (`transcript` o `captions_url`) y sin `alt_text`.
`LoadDir` SHALL loguear los huecos. Con `AVLP_REQUIRE_MEDIA_A11Y=true` SHALL
rechazar la carga.

#### Scenario: Reporte con huecos

- **GIVEN** un nodo con variante video sin transcript ni captions
- **WHEN** se genera el reporte
- **THEN** incluye la ubicación de esa variante

#### Scenario: Flag estricto

- **GIVEN** `AVLP_REQUIRE_MEDIA_A11Y=true` y un nodo con huecos
- **WHEN** se intenta registrar el nodo
- **THEN** la operación falla con error de accesibilidad

### Requirement: Promoción copia el contenido a transcript

`PromoteLiveStation` SHALL asignar el contenido markdown de la estación lista
al `transcript` del nodo curado.

#### Scenario: Primera promoción

- **GIVEN** una estación ready con contenido
- **WHEN** se promueve
- **THEN** el seed persistente incluye ese texto en `transcript`

### Requirement: Stage expone alternativas al estudiante

La UI SHALL mostrar `alt_text`, un toggle «Ver transcripción» cuando hay
`transcript`, y un `<track kind="captions">` cuando hay `captions_url` en un
`<video>`.

#### Scenario: Toggle de transcripción

- **GIVEN** un nodo interactivo con transcript
- **WHEN** el estudiante activa el toggle
- **THEN** la transcripción se muestra debajo del Stage con `aria-expanded`
