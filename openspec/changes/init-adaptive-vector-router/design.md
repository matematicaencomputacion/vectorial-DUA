# Technical Design: Motor de Ruteo Vectorial DUA (Go + gRPC)

## Resumen técnico

Construcción de un microservicio concurrente en Go que actúa como el núcleo del espacio vectorial adaptativo. Utiliza estructuras en memoria ($k\text{-NN}$) para resolver las consultas del Agente en milisegundos y coordinar las vistas de la Pantalla Master y el IDE mediante gRPC.

## Componentes e Interfaces

```text
[ Agente / IDE ]
        │
   (gRPC / Protobuf)
        │
        ▼
┌─────────────────────────────────────────────────────────┐
│                  Core Router (Go)                       │
│  - gRPC Server (router_api.proto) :50051                │
│  - Vector Engine (Distancia Coseno / k-NN in-memory)    │
│  - ULID Indexer & Consistent Hashing Ring               │
│  - In-process Event Bus (NodeNotFoundEvent)             │
└─────────────────────────────────────────────────────────┘
        │                                   │
 (Similitud ≥ 0.85)                 (Similitud < 0.85)
        │                                   │
        ▼                                   ▼
[ Devuelve Nodo DUA ]             [ Emite NodeNotFound ]
(Estático / Master)               (Generador En Vivo)
```

## Contratos de Protocol Buffer

### `student_state.proto`

```protobuf
syntax = "proto3";
package avlp.vector.v1;
option go_package = "github.com/vectorial-dua/avlp/gen/avlp/vector/v1;vectorv1";

message StudentVector {
  string student_id = 1;
  repeated float dimensions = 2; // [Dominio, Sensorial, Frustracion, Ritmo, Autonomia]
  int64 timestamp = 3;
}

message VectorQuery {
  StudentVector student_state = 1;
  repeated float query_embedding = 2;
  float min_similarity_threshold = 3; // 0 => default 0.85
}
```

### `node_schema.proto`

```protobuf
syntax = "proto3";
package avlp.vector.v1;
option go_package = "github.com/vectorial-dua/avlp/gen/avlp/vector/v1;vectorv1";

message NodeRecord {
  string node_id = 1;           // dua::<dim>::<dif>::<fmt>::<ulid>
  string dimension_dua = 2;     // Representacion | Accion | Compromiso
  string difficulty = 3;
  string format = 4;            // visual | conceptual | practica
  string resource_url = 5;
  repeated float embedding = 6;
}
```

### `router_api.proto`

```protobuf
syntax = "proto3";
package avlp.vector.v1;
option go_package = "github.com/vectorial-dua/avlp/gen/avlp/vector/v1;vectorv1";

import "student_state.proto";

service VectorRouter {
  rpc QueryNearestNode(VectorQuery) returns (RouteResult);
}

message NodeResponse {
  string node_id = 1;
  string dimension_dua = 2;
  string resource_url = 3;
  float similarity_score = 4;
  bool is_live_generated = 5;
}

message LiveStationPending {
  string tracking_ulid = 1;
  string status = 2; // "in_progress"
  string message = 3;
}

message RouteResult {
  oneof outcome {
    NodeResponse matched = 1;
    LiveStationPending pending = 2;
  }
}
```

### `events.proto`

```protobuf
syntax = "proto3";
package avlp.vector.v1;
option go_package = "github.com/vectorial-dua/avlp/gen/avlp/vector/v1;vectorv1";

import "student_state.proto";

message NodeNotFoundEvent {
  string event_id = 1;
  string tracking_ulid = 2;
  string student_id = 3;
  repeated float query_embedding = 4;
  float best_similarity = 5;
  float threshold = 6;
  int64 timestamp = 7;
}
```

## Layout de paquetes Go

- `proto/` — fuentes `.proto`
- `gen/avlp/vector/v1/` — stubs generados
- `pkg/vector/` — coseno, ULID, índice k-NN, bus de eventos
- `cmd/router/` — servidor gRPC
- `cmd/router-client/` — cliente de prueba concurrente

## Métricas y Rendimiento Esperado

- Tiempo de respuesta p99: $< 15\text{ms}$.
- Concurrencia: soporte de miles de goroutines concurrentes sobre el índice con `RWMutex`.
- Throughput coseno: $> 100{,}000$ ops/sec en benchmarks unitarios.
