#!/usr/bin/env bash
# gofmt gate. A script rather than inline workflow steps so it can be run
# locally, and so the directory list lives in one place.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

unformatted="$(gofmt -l cmd internal)"
if [[ -n "$unformatted" ]]; then
  echo "not gofmt-clean:"
  echo "$unformatted"
  gofmt -d cmd internal
  exit 1
fi
echo "gofmt clean"
