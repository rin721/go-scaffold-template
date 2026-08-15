#!/usr/bin/env bash
set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${repository_root}"

if [[ -n "$(gofmt -l .)" ]]; then
  gofmt -d .
  exit 1
fi
go mod tidy -diff
go generate ./...
git diff --exit-code -- api internal/transport/http/api
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
CGO_ENABLED=0 go build -trimpath -buildvcs=false ./...
bash scripts/verify-artifacts.sh
