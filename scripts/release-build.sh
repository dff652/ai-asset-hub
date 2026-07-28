#!/usr/bin/env bash
# Build the release artifacts for every supported platform, plus SHA256SUMS.
#
# The release workflow is a thin wrapper around this script on purpose: a
# workflow YAML cannot be run locally, this can. See docs/development.md 2.3.
#
# Usage:
#   VERSION=0.1.0 ./scripts/release-build.sh            # -> dist/release/
#   VERSION=0.1.0 OUT=/tmp/rel ./scripts/release-build.sh
#
# Cross-compilation only proves the code builds for a platform. Per ADR-0003
# section 4 that is not the same as verifying its behaviour there.

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

PLATFORMS=(
  "linux/amd64"
  "linux/arm64"
  "darwin/amd64"
  "darwin/arm64"
  "windows/amd64"
  "windows/arm64"
)

rm -rf "$OUT"
mkdir -p "$OUT"
echo "==> building aiah $VERSION"

for platform in "${PLATFORMS[@]}"; do
  goos="${platform%/*}"
  goarch="${platform#*/}"
  name="aiah_${VERSION}_${goos}_${goarch}"
  binary="$name"
  [[ "$goos" == "windows" ]] && binary="${name}.exe"

  GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 \
    go build -trimpath -ldflags "$LDFLAGS" -o "$OUT/$binary" ./cmd/aiah
  echo "    $binary"
done

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
ls -1 "$OUT"

# The host-platform binary must report the version it was stamped with; a
# release that cannot identify itself is the gap this whole pipeline closes.
host_binary="$OUT/aiah_${VERSION}_$(go env GOOS)_$(go env GOARCH)"
if [[ -x "$host_binary" ]]; then
  echo "==> self check"
  reported="$("$host_binary" version)"
  echo "    $reported"
  case "$reported" in
    *"$VERSION"*) ;;
    *) echo "error: binary reports '$reported', expected to contain '$VERSION'" >&2; exit 1 ;;
  esac
fi
