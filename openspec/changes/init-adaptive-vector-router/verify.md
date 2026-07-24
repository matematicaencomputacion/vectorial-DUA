# Verification: init-adaptive-vector-router

Fecha: 2026-07-24

## Escenarios Given/When/Then

| Escenario | Resultado | Evidencia |
|-----------|-----------|-----------|
| Redirección a nodo estático (≥ 0.85) | PASS | `TestRouterStaticMatchAboveThreshold`; probe gRPC `similarity_score:1` → `dua::Representacion::basico::visual::...` |
| Disparo Estación en Vivo (< 0.85) | PASS | `TestRouterLiveStationOnMiss` emite `NodeNotFoundEvent` + `tracking_ulid` / `in_progress` |
| Generación/validación ULID jerárquico | PASS | `TestNodeIDFormatAndChronologicalOrder`, `TestIndexUniqueULIDRing` |

## Rendimiento

- Benchmark coseno: ~40M ops/sec (objetivo > 100k) — `BenchmarkCosineSimilarity`
- Throughput test: `TestCosineThroughputTarget` PASS
- Cliente concurrente: 1000 req / 64 workers modo `match` sobre `:50051`

## Comandos

```bash
go test ./...
go run ./cmd/router
go run ./cmd/router-client -mode match -n 1000 -c 64
go run ./cmd/router-client -mode miss -n 100 -c 16
```
