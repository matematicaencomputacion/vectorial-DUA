# Delta Spec: Motor de Ruteo Vectorial y Asignación DUA

## ADDED Requirements

### Requirement: Cálculo de Distancia Coseno y Ruteo de Nodos DUA

El sistema SHALL recibir una petición gRPC con el vector de la duda del alumno y encontrar el nodo DUA más cercano en el espacio vectorial $\mathbb{R}^N$.

#### Scenario: Redirección exitosa a un nodo estático preexistente

- GIVEN un vector de consulta del estudiante $V_e$ con coordenadas definidas
- AND un índice de nodos $k\text{-NN}$ en memoria
- WHEN el servicio de ruteo ejecuta la búsqueda vectorial
- AND la similitud coseno con el nodo más cercano es mayor o igual al umbral ($\ge 0.85$)
- THEN el sistema devuelve el nodo DUA correspondiente identificado por su ULID
- AND retorna la estrategia DUA asignada (representación visual, práctica o conceptual)

#### Scenario: Disparo de generación de Nodo "En Vivo" por espacio vacío

- GIVEN un vector de consulta del estudiante $V_e$
- WHEN el servicio de ruteo ejecuta la búsqueda en el espacio vectorial
- AND ningún nodo preexistente alcanza el umbral de similitud coseno ($< 0.85$)
- THEN el sistema emite un evento `NodeNotFoundEvent` vía bus de eventos
- AND devuelve un estado de respuesta en progreso con un token de seguimiento ULID para la Estación en Vivo

### Requirement: Direccionamiento e Identificación Única de Nodos

El sistema SHALL nombrar e indexar cada nodo vectorial usando la especificación jerárquica basada en ULID ordenados por tiempo.

#### Scenario: Generación y validación del identificador ULID

- GIVEN la solicitud de creación o registro de un nuevo nodo DUA
- WHEN el motor procesa el registro
- THEN genera una clave con la estructura `dua::<dimension>::<dificultad>::<formato>::<ulid>`
- AND verifica que el ULID sea ordenable cronológicamente y único en el anillo de hashing
