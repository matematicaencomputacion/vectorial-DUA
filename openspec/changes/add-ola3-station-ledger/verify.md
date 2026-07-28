# Verify: Ola 3.a — StationLedger / GetLiveStation (C2)

## Checklist

- [x] `StationLedger` registra `in_progress | ready | failed` por `tracking_ulid`
- [x] Lazy retry con guard `Retrying` (polls concurrentes → un solo `GenerateLive`)
- [x] TTL `AVLP_STATION_TTL` (default 24h)
- [x] RPC `GetLiveStation` con `student_message` rogeriano en español
- [x] NotFound uniforme para ULID ausente y `student_id` incorrecto
- [x] `go test -race ./...` verde en merge a `main`
- [x] Tag anotado `v0.3.0-ola3a`
