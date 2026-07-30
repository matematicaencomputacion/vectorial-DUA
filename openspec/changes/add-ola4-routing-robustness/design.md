## Context

El umbral estático se resolvía por request/env/default y `calibrate` solo
producía un reporte. `HashEmbedder` y `HTTPEmbedder` trataban el texto de forma
distinta, mientras `simmatrix` observaba nodos pero no chunks. No cambia ningún
contrato Protobuf ni superficie gRPC.

## Goals / Non-Goals

**Goals:**

- Hacer aplicable y auditable la calibración sin contaminar los tests.
- Normalizar en un único choke point que cubra queries, seeds, chunks y live.
- Medir el piso RAG con evidencia completa, incluso cuando no hay corte limpio.
- Mantener el costo de normalización lineal en el tamaño del texto y fuera del
  camino de coseno, preservando el objetivo de latencia p99 < 15 ms.

**Non-Goals:**

- Hot reload de configuración, autocorrección ortográfica o escritura
  automática de `AVLP_RAG_MIN_SIMILARITY`.

## Decisions

### Config JSON versionada y escritura atómica

`calibrate --apply` escribe `{version, similarity_threshold}` mediante temporal,
`Sync` y `Rename`. El router usa `data/avlp.json` como ruta default;
`AVLP_CONFIG_PATH` permite reemplazarla. La librería solo consulta archivo si
recibe una ruta, evitando que estado local ignorado altere tests.

Alternativa descartada: editar `.env`; mezcla secretos y estado de calibración,
es difícil de parsear sin preservar comentarios y no ofrece actualización
atómica simple.

### Captura del umbral al construir el router

El proceso resuelve `AVLP_SIMILARITY_THRESHOLD` > archivo > `0.85`, registra
valor/origen y lo asigna al router. El umbral explícito válido de cada RPC sigue
ganando. Capturarlo evita cambios silenciosos por editar el archivo en vuelo.

### Normalización dentro de ambos embedders

La canonicalización ocurre dentro de `Embed`, no en cada call site: lowercase,
descomposición Unicode, descarte de marcas diacríticas y colapso de letras
consecutivas iguales. Así todos los productores comparten la regla, incluyendo
ingesta RAG y generación live. `golang.org/x/text/unicode/norm` evita tablas
manuales incompletas.

El typo no fonético `escopes` se registra además como lenguaje observado en el
descriptor del seed de Variables y Scope; no se introduce un corrector general.

### Matriz RAG separada en el mismo comando

`simmatrix` conserva `simmatrix.json` query×nodo y añade
`simmatrix_rag.json` query×chunk. El store expone un snapshot profundo en orden
de inserción, por lo que la matriz no mantiene locks durante el cómputo y puede
ejecutarse concurrentemente con lectores sin carreras.

El piso sugerido es el punto medio entre el peor máximo de fuente esperada
on-topic y el mejor máximo off-topic. Un margen no positivo se reporta, no se
disimula ni se aplica automáticamente.

## Risks / Trade-offs

- **Colapsar dobles legítimos puede reducir información léxica** → se aplica
  simétricamente y se cubre con goldens; un embedder semántico atenúa el riesgo.
- **`--apply` puede persistir una sugerencia con warning** → el warning queda
  visible en consola y reporte; el operador conserva el acto explícito.
- **Hash no separa todo el corpus RAG con un piso único** → la matriz documenta
  el solapamiento; no se eleva el default perdiendo PostGIS.
- **Config corrupta** → fallback no fatal a default con origen registrado.

## Migration Plan

1. Desplegar sin `data/avlp.json`: comportamiento efectivo sigue en `0.85`.
2. Ejecutar simmatrix/calibrate con el embedder operativo y revisar warnings.
3. Aplicar el umbral o usar env para override inmediato.
4. Para rollback, borrar el archivo o fijar la variable al valor anterior.
