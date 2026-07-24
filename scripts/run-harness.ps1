$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
Set-Location $Root

$suite = if ($args[0]) { $args[0] } else { "evals" }

Write-Host "==> go test ./..."
go test ./...
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "==> harness suite=$suite"
go run ./cmd/harness -suite $suite @args[1..($args.Length)]
