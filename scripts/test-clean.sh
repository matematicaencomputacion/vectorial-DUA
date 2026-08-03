#!/usr/bin/env bash
# Pre-push test run with no AVLP_* variables inherited from the shell.
#
# Routing reads AVLP_* at run time, so a shell with (for example)
# AVLP_SIMILARITY_THRESHOLD=0.55 exported can turn a real regression green.
# The test suite clears these itself (internal/testenv); this script is the
# operator-side guard so a stale export never reaches the toolchain either.
#
# Usage: ./scripts/test-clean.sh [extra go test args...]
#
# Note: `env -i` is avoided on purpose — it also drops HOME and breaks the Go
# build cache.
set -euo pipefail

cd "$(dirname "$0")/.."

for key in $(env | sed -n 's/^\(AVLP_[A-Za-z0-9_]*\)=.*/\1/p'); do
  echo "limpiando $key"
  unset "$key"
done

leaked=$(env | grep -c '^AVLP_' || true)
if [ "$leaked" -ne 0 ]; then
  echo "error: quedaron $leaked variables AVLP_* en el entorno" >&2
  exit 1
fi

echo "==> go build ./..."
go build ./...

echo "==> go vet ./..."
go vet ./...

# Guardado explícito: bash 3.2 (macOS) trata "$@" vacío como unbound bajo set -u.
if [ "$#" -gt 0 ]; then
  echo "==> go test -race -count=1 ./... $*"
  go test -race -count=1 ./... "$@"
else
  echo "==> go test -race -count=1 ./..."
  go test -race -count=1 ./...
fi
