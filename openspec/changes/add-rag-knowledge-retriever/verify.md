# Verification: add-rag-knowledge-retriever

Fecha: 2026-07-24

## Escenarios

| Escenario | Resultado | Evidencia |
|-----------|-----------|-----------|
| Ingest knowledge base | PASS | `TestIngestAndRetrieveEnvDoc` |
| Retrieve alineado a .env | PASS | top hit `env-variables.md` |
| Miss → live matched + sources | PASS | `TestGenerateLiveStationWithSources` |
| RAG disabled → pending | PASS | `TestRouterLiveStationOnMiss` (Live=nil) |
| Routing golden + live RAG | PASS | `TestGoldenRoutingEvalsPass` |
| Faithfulness golden | PASS | `TestRAGFaithfulnessGolden` |

## Comandos

```powershell
go test ./pkg/rag/... ./pkg/livestation/... ./pkg/rogerian/... ./harness/evals/... ./pkg/vector/...
go run ./cmd/harness -suite evals
go run ./cmd/router   # indexa data/knowledge_base
```
