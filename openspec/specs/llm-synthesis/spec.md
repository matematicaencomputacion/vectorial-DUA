# Spec: LLM Synthesis

## Purpose

Permite convertir contexto RAG verificado en explicaciones DUA generativas y
rogerianas, manteniendo atribución de fuentes y un camino offline explícito.

## Requirements

### Requirement: Síntesis generativa opcional

El sistema SHALL poder sintetizar una estación live mediante un backend
OpenAI Chat Completions compatible usando el `PromptBundle` grounded.

#### Scenario: Backend configurado

- **GIVEN** un synthesizer configurado y contexto RAG recuperado
- **WHEN** se genera una estación live
- **THEN** el contenido de la estación es la síntesis del `FullPrompt`
- **AND** el contrato Markdown existente se conserva

#### Scenario: Modo offline

- **GIVEN** ningún backend de síntesis configurado
- **WHEN** se genera una estación live
- **THEN** el sistema usa el renderer extractivo
- **AND** registra explícitamente que el fallback está activo

#### Scenario: Fallo del backend

- **GIVEN** un backend configurado que devuelve error o excede el timeout
- **WHEN** se genera una estación live
- **THEN** la estación se completa mediante fallback extractivo
- **AND** el error y el fallback quedan registrados sin exponer detalles al estudiante

### Requirement: Cliente HTTP configurable y resiliente

El synthesizer HTTP SHALL aceptar URL, modelo, API key y timeout por
configuración, y SHALL reintentar respuestas 5xx transitorias.

#### Scenario: Respuesta exitosa

- **GIVEN** un backend OpenAI-compatible disponible
- **WHEN** responde con una choice de contenido no vacío
- **THEN** el synthesizer devuelve ese Markdown

#### Scenario: Error no recuperable

- **GIVEN** una respuesta 4xx, JSON inválido o choices vacías
- **WHEN** se solicita síntesis
- **THEN** el synthesizer devuelve un error claro y no reintenta errores 4xx

#### Scenario: Error transitorio

- **GIVEN** una primera respuesta 5xx seguida de una respuesta válida
- **WHEN** se solicita síntesis
- **THEN** el synthesizer reintenta y devuelve la respuesta válida

### Requirement: Grounding y atribución controlada por la aplicación

El prompt SHALL exigir uso exclusivo del contexto citado y la aplicación SHALL
agregar al final una sección `Fuentes` derivada de los chunks recuperados.

#### Scenario: Fuentes presentes

- **GIVEN** una síntesis y fuentes RAG recuperadas
- **WHEN** se materializa el contenido final
- **THEN** termina con una lista `Fuentes` generada por la aplicación
- **AND** no depende de que el modelo produzca esa lista

#### Scenario: Sin contexto recuperado

- **GIVEN** una estación sin hits RAG
- **WHEN** se construye el prompt
- **THEN** ordena declarar la falta de información y prohíbe completar con conocimiento externo

### Requirement: Evaluación offline de contenido generativo

El harness SHALL ofrecer un modo de faithfulness generativo sin invocar un
juez remoto, separado de la métrica extractiva por substring.

#### Scenario: Respuesta grounded

- **GIVEN** una respuesta generativa con términos clave soportados y la fuente esperada
- **WHEN** se evalúa en modo generativo
- **THEN** informa precisión grounded, cobertura léxica y aggregate

#### Scenario: Afirmaciones sin soporte

- **GIVEN** una respuesta dominada por términos ausentes del contexto
- **WHEN** se evalúa en modo generativo
- **THEN** obtiene menor puntaje de fidelidad aunque cite una fuente
