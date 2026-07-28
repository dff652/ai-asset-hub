#!/usr/bin/env bash
# Build aiah with its version stamped in.
#
# A plain `go build ./cmd/aiah` also works and yields version "dev", which is
# the honest answer for an unreleased binary. Use this script whenever the
# binary might end up somewhere its identity matters: releases, or any install
# whose deployment record you want to trace back to a commit.
#
# Usage:
#   ./scripts/build.sh                    # -> build/aiah
#   VERSION=0.1.0 ./scripts/build.sh      # explicit release version
#   OUT=/tmp/aiah ./scripts/build.sh      # custom output path

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

OUT="${OUT:-$ROOT/build/aiah}"
# shellcheck source=scripts/_stamp.sh
source "$ROOT/scripts/_stamp.sh"

mkdir -p "$(dirname "$OUT")"
go build -trimpath -ldflags "$LDFLAGS" -o "$OUT" ./cmd/aiah

echo "built $OUT"
"$OUT" version
