# Design: Progreso jerárquico

## Semántica

El progreso se calcula sobre todos los nodos del árbol, incluidas raíces y
descendientes.

- `total_subtopics`: cantidad recursiva de subtemas.
- `opened_subtopic_ids`: intersección entre lo registrado y los IDs del árbol,
  en orden de recorrido pre-order para una respuesta estable.
- Estado de cada raíz:
  - `unvisited`: no se abrió ningún subtema de su subárbol.
  - `visited`: se abrieron todos los subtemas de su subárbol.
  - `partial`: se abrió al menos uno, pero no todos.

Así, abrir `Motor` marca `Motor` como visitado y `Caja Central` como parcial;
abrir una raíz hoja como `4 Ruedas` marca esa raíz como visitada.

## Contrato

`GetSubtopicProgress(SubtopicProgressQuery) returns (SubtopicProgress)`

La respuesta incluye identidad, IDs abiertos, total recursivo y estados de las
raíces. Los estados viajan como strings para mantener JSON legible y evitar que
un enum desconocido se degrade silenciosamente a cero.

## Validación

- `student_id` o `parent_node_id` vacío: `InvalidArgument`.
- Nodo inexistente: `NotFound`.
- Nodo sin `hierarchy`: `NotFound`.

## Persistencia

PR 7.1 expone el `InteractionStore` actual. Su vida sigue ligada al proceso del
router; reiniciar solo `master-web` conserva el progreso, reiniciar el router
no. Persistencia durable queda fuera de C4.
