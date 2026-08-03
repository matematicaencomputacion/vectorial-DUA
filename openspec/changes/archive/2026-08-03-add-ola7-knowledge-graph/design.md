## Context

Las aristas viven entre **conceptos** (`concept:<slug>`), no entre `node_id`:
SeedDemoNodes regenera ULIDs, las estaciones live nacen en runtime, y un mismo
concepto DUA se enseña con varios nodos (Representación / Acción / Compromiso).

## Decisions

### 1. Dirección única

La flecha apunta al concepto más fundacional/anterior: `from --requires--> to`
se lee «from se apoya en to». `Path` devuelve orden de aprendizaje (reverso del
recorrido hacia lo fundacional).

### 2. Ciclos

Se detectan por kind en `requires`, `deepens` y `continues` (DFS con colores).
`alternative` es simétrico y no entra al chequeo de ciclos.

### 3. Validación vs avisos

Errores duros abortan la carga. Avisos van al `Report`/`Logf`. Con
`AVLP_KNOWLEDGE_STRICT=true`, concepto sin recurso y recurso con concepto
ausente fallan (espíritu de `AVLP_REQUIRE_MEDIA_A11Y`).

### 4. Binding

`IndexBinder{Index, Registry}` deriva concepto↔recurso en cada arranque desde
`Concepts` declarados en seeds y nodos demo.

### 5. Firmas Neo4j-ready

`KnowledgeGraph` y `ResourceBinder` llevan `context.Context` y `error`.
Prerequisites/Dependents/Neighbors devuelven `[]Relation` (arista + peer + Depth)
con `TraverseOptions`. `Health(ctx) error` es sonda de disponibilidad;
cobertura y conteos viven en `Stats()`.

## Risks

- Rationale de borrador mal curado llega al estudiante → marcado en proposal.
- Grafo incompleto → Stats/avisos, no bloqueo por defecto.
