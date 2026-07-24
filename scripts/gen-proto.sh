#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
mkdir -p gen/avlp/vector/v1
protoc -I proto \
  --go_out=gen --go_opt=module=github.com/vectorial-dua/avlp/gen \
  --go-grpc_out=gen --go-grpc_opt=module=github.com/vectorial-dua/avlp/gen \
  proto/student_state.proto proto/node_schema.proto proto/router_api.proto proto/events.proto proto/harness_eval.proto
echo "protobuf stubs generated under gen/"
