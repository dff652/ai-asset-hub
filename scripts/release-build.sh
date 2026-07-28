#!/usr/bin/env bash
# Build the supported Linux amd64 release artifact, plus SHA256SUMS.
#
# The release workflow is a thin wrapper around this script on purpose: a
# workflow YAML cannot be run locally, this can. See docs/development.md 2.3.
#
# Usage:
#   VERSION=0.1.0 ./scripts/release-build.sh            # -> dist/release/
#   VERSION=0.1.0 OUT=/tmp/rel ./scripts/release-build.sh
#
# Other targets remain in CI as build-health checks. They are not distributed
# until they have native acceptance coverage.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

# Release binaries drop the symbol table; identity still comes from _stamp.sh.
EXTRA_LDFLAGS="-s -w"
# shellcheck source=scripts/_stamp.sh
source "$ROOT/scripts/_stamp.sh"
# shellcheck source=scripts/_sha256.sh
source "$ROOT/scripts/_sha256.sh"
OUT="${OUT:-$ROOT/dist/release}"

rm -rf "$OUT"
mkdir -p "$OUT"
echo "==> building aiah $VERSION"

binary="aiah_${VERSION}_linux_amd64"
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
  go build -trimpath -ldflags "$LDFLAGS" -o "$OUT/$binary" ./cmd/aiah
echo "    $binary"

# Binary recipients need the project license, NOTICE, dependency inventory, and
# the complete third-party license texts alongside the executable.
cp "$ROOT/LICENSE" "$OUT/LICENSE"
cp "$ROOT/NOTICE" "$OUT/NOTICE"
cp "$ROOT/THIRD_PARTY_LICENSES.txt" "$OUT/THIRD_PARTY_LICENSES.txt"
cp "$ROOT/docs/licenses/third-party.md" "$OUT/THIRD_PARTY_DEPENDENCIES.md"

# Checksums cover both executables and the compliance material shipped beside
# them.
(
  cd "$OUT"
  sha256_write \
    aiah_* \
    LICENSE \
    NOTICE \
    THIRD_PARTY_LICENSES.txt \
    THIRD_PARTY_DEPENDENCIES.md >SHA256SUMS
)

echo "==> artifacts in $OUT"
"$ROOT/scripts/check-release-checksums.sh" "$OUT"
ls -1 "$OUT"

# The host-platform binary must report the version it was stamped with; a
# release that cannot identify itself is the gap this whole pipeline closes.
host_binary="$OUT/$binary"
if [[ "$(go env GOOS)/$(go env GOARCH)" == "linux/amd64" ]]; then
  echo "==> self check"
  reported="$("$host_binary" version)"
  echo "    $reported"
  case "$reported" in
    *"$VERSION"*) ;;
    *) echo "error: binary reports '$reported', expected to contain '$VERSION'" >&2; exit 1 ;;
  esac
fi
