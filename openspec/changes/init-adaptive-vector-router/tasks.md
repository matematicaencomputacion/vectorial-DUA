# Tasks: Inicialización del Motor de Ruteo Vectorial DUA

## 1. Definición de Contratos y Entorno (OpenSpec)

- [x] Validar la estructura del cambio en `openspec/changes/init-adaptive-vector-router/`.
- [x] Compilar archivos `.proto` en el directorio `/proto` para Go (`protoc-gen-go-grpc`).

## 2. Core del Engine en Go (`/pkg/vector`)

- [x] Crear paquete `vector` con funciones optimizadas para cálculo de distancia coseno.
- [x] Implementar generador y validador de identificadores ULID (`dua::<dim>::<dif>::<fmt>::<ulid>`).
- [x] Implementar estructura de datos en memoria para almacenamiento de nodos y búsqueda $k\text{-NN}$.

## 3. Servidor gRPC (`/cmd/router`)

- [x] Implementar el servidor gRPC de Go escuchando en el puerto `:50051`.
- [x] Implementar el handler `QueryNearestNode` que consume `VectorQuery`.
- [x] Añadir lógica de umbral de corte ($\ge 0.85$ para éxito, $< 0.85$ para fallback).
- [x] Integrar emisor de eventos para el caso `NodeNotFound`.

## 4. Pruebas y Validación

- [x] Agregar pruebas unitarias de rendimiento para el cálculo de distancia coseno en Go ($> 100,000$ ops/sec).
- [x] Crear cliente de prueba en Go para simular peticiones concurrentes de estudiantes.
- [x] Ejecutar verificación SDD de `init-adaptive-vector-router` para confirmar cumplimiento de la especificación.
