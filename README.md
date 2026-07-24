# AVLP — Adaptive Vector Learning Platform (vectorial-DUA)

Plataforma educativa vectorial y adaptativa (**DUA** + **Carl Rogers**), con **OpenSpec (SDD)**, **Harness** y **RAG** para estaciones en vivo ancladas a conocimiento verificado.

## Triángulo del entorno adaptativo

| Capa | Rol | Estado en repo |
|------|-----|----------------|
| **Master** | Mapa de rutas DUA, nodos estáticos / en vivo | Contrato + seeds (UI futura) |
| **IDE (Antigravity)** | Ejecución y experimentación en celdas | Sandbox harness |
| **Agente** | Scaffolding rogeriano + RAG | `pkg/rogerian` + `pkg/rag` |

## Flujo RAG

```text
Duda → Router k-NN
  ├─ sim ≥ 0.85 → Nodo DUA estático
  └─ sim < 0.85 → RAG retrieve → Prompt Rogers → Estación en vivo (ULID)
```

Flag: `AVLP_RAG_ENABLED` (default `true`). Knowledge base: `AVLP_KB_ROOT` o `data/knowledge_base`.

## Árbol del repositorio

```text
vectorial-DUA/
├── cmd/{router,router-client,harness}
├── pkg/{vector,rag,livestation,dua,rogerian}
├── data/knowledge_base/{henry,postgis,node}
├── proto/ + gen/
├── harness/{evals,sandbox,load,telemetry}
├── openspec/changes/
│   ├── init-adaptive-vector-router/
│   ├── init-harness-and-vector-router/
│   └── add-rag-knowledge-retriever/
└── scripts/
```

## OpenSpec / SDD

1. `init-adaptive-vector-router` — ruteo vectorial  
2. `init-harness-and-vector-router` — harness  
3. `add-rag-knowledge-retriever` — RAG + estaciones en vivo  

## Guía de ejecución

```powershell
go test ./...
go run ./cmd/router
go run ./cmd/harness -suite evals
go run ./cmd/harness -suite sandbox
go run ./cmd/harness -suite load -n 300 -c 24
```

Regenerar Protobuf: `./scripts/gen-proto.ps1`

## SLOs

| Métrica | Objetivo |
|---------|----------|
| Ruteo / retrieve in-process p99 | &lt; 15ms |
| Eval routing + RAG faithfulness | pass_rate / aggregate ≥ 0.80 |
| Load error rate | &lt; 1% |
