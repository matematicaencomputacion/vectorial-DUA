## 1. Sesión y gateway

- [x] 1.1 pkg/session: emitir/verificar token HMAC + env helpers
- [x] 1.2 POST /api/session + Bearer en handlers; metadata gRPC
- [x] 1.3 Wiring master-web / logs de modo abierto vs seguro

## 2. Router y promoción

- [x] 2.1 routerserver: exigir metadata en modo seguro; IDOR NotFound
- [x] 2.2 PromoteLiveStation exige teacher; mensaje neutro

## 3. UI y verificación

- [x] 3.1 Sesión al cargar, token en memoria, Soy docente, botón promover
- [x] 3.2 Tests + Playwright abierto/seguro + README; PR con freno
