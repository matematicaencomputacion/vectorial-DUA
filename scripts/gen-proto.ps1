$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
Set-Location $Root

$protoc = Get-Command protoc -ErrorAction SilentlyContinue
if (-not $protoc) {
  $candidate = Get-ChildItem -Path "$env:LOCALAPPDATA\Microsoft\WinGet\Packages" -Recurse -Filter protoc.exe -ErrorAction SilentlyContinue |
    Select-Object -First 1 -ExpandProperty FullName
  if (-not $candidate) { throw "protoc not found in PATH" }
  $protocExe = $candidate
} else {
  $protocExe = $protoc.Source
}

New-Item -ItemType Directory -Force -Path "gen\avlp\vector\v1" | Out-Null
$protos = @(
  "proto/student_state.proto",
  "proto/node_schema.proto",
  "proto/router_api.proto",
  "proto/events.proto",
  "proto/harness_eval.proto"
)

& $protocExe -I proto `
  --go_out=gen --go_opt=module=github.com/vectorial-dua/avlp/gen `
  --go-grpc_out=gen --go-grpc_opt=module=github.com/vectorial-dua/avlp/gen `
  @protos

Write-Host "protobuf stubs generated under gen/"
