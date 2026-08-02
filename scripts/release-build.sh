#!/usr/bin/env bash
# Build the distributed release artifacts, plus SHA256SUMS.
#
# The release workflow is a thin wrapper around this script on purpose: a
# workflow YAML cannot be run locally, this can. See docs/development.md 2.3.
#
# Usage:
#   VERSION=0.1.0 ./scripts/release-build.sh            # -> dist/release/
#   VERSION=0.1.0 OUT=/tmp/rel ./scripts/release-build.sh
#
# What ships is decided by acceptance coverage, not by what compiles. linux/amd64
# and darwin/arm64 both run the full suite -- tests, vet, gofmt, the fake-HOME
# closed loop and the installer regression -- on their own runner. darwin/amd64
# ships alongside arm64 because a pure-Go CGO_ENABLED=0 binary differs only in
# codegen across architectures of the same OS, and macOS semantics are verified
# on arm64; that is a weaker claim than the other two and the docs say so.
#
# The remaining build-matrix targets stay CI-only build-health checks. Shipping a
# binary reads as a support claim, and Windows chmod, shebang and config-root
# semantics have no acceptance coverage at all.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

# Release binaries drop the symbol table; identity still comes from _stamp.sh.
EXTRA_LDFLAGS="-s -w"
# shellcheck source=scripts/_stamp.sh
source "$ROOT/scripts/_stamp.sh"
# shellcheck source=scripts/_sha256.sh
source "$ROOT/scripts/_sha256.sh"
# shellcheck source=scripts/_release_platforms.sh
source "$ROOT/scripts/_release_platforms.sh"
OUT="${OUT:-$ROOT/dist/release}"

rm -rf "$OUT"
mkdir -p "$OUT"
echo "==> building aiah $VERSION"

for platform in "${RELEASE_PLATFORMS[@]}"; do
  goos="${platform%/*}"
  goarch="${platform#*/}"
  binary="aiah_${VERSION}_${goos}_${goarch}"
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
"$ROOT/scripts/check-release-checksums.sh" "$OUT"
ls -1 "$OUT"

# Only the host-platform binary can be executed here, so only it can be asked
# whether it knows its own version. Cross-built artifacts are covered by their
# own runner's suite, not by this check.
host_platform="$(go env GOOS)/$(go env GOARCH)"
host_binary="$OUT/aiah_${VERSION}_$(go env GOOS)_$(go env GOARCH)"
if [[ " ${RELEASE_PLATFORMS[*]} " == *" $host_platform "* ]]; then
  echo "==> self check ($host_platform)"
  reported="$("$host_binary" version)"
  echo "    $reported"
  case "$reported" in
    *"$VERSION"*) ;;
    *) echo "error: binary reports '$reported', expected to contain '$VERSION'" >&2; exit 1 ;;
  esac
else
  echo "==> self check skipped: no artifact for host $host_platform"
fi
