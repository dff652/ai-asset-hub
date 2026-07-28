#!/usr/bin/env bash
# Full local gate used before a commit or release rehearsal.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

"$ROOT/scripts/dev-doctor.sh"

bash -n "$ROOT"/scripts/*.sh
"$ROOT/scripts/test-dev-doctor.sh"
"$ROOT/scripts/generate-third-party-licenses.sh" --check
"$ROOT/scripts/test-release-checksums.sh"
go test ./...
go test -race ./...
go vet ./...
"$ROOT/scripts/check-gofmt.sh"
golangci-lint run ./...
"$ROOT/scripts/demo-apply-scan-loop.sh"
