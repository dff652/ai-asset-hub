#!/usr/bin/env bash
# Regression test: a similarly named checksum entry must not cover a license.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fixture="$(mktemp -d)"
cleanup() {
  rm -rf "$fixture"
}
trap cleanup EXIT

for name in LICENSE NOTICE THIRD_PARTY_LICENSES.txt THIRD_PARTY_DEPENDENCIES.md; do
  printf '%s\n' "$name fixture" >"$fixture/$name"
done

# Keep the adversarial input internally coherent: the decoy exists and its
# checksum is valid. Only the exact required LICENSE entry is absent.
printf '%s\n' "decoy" >"$fixture/LICENSE.decoy"
printf '%s\n' "linux binary" >"$fixture/aiah_0.1.1_linux_amd64"
# shellcheck source=scripts/_sha256.sh
source "$ROOT/scripts/_sha256.sh"
(
  cd "$fixture"
  sha256_write \
    aiah_0.1.1_linux_amd64 \
    LICENSE.decoy \
    NOTICE \
    THIRD_PARTY_LICENSES.txt \
    THIRD_PARTY_DEPENDENCIES.md >SHA256SUMS
)

if "$ROOT/scripts/check-release-checksums.sh" "$fixture" >/dev/null 2>&1; then
  echo "error: LICENSE.decoy was accepted as the LICENSE checksum entry" >&2
  exit 1
fi

# A complete Linux amd64 release passes, while adding an unvalidated platform
# artifact makes the same release invalid even when its checksum is correct.
(
  cd "$fixture"
  sha256_write \
    aiah_0.1.1_linux_amd64 \
    LICENSE \
    NOTICE \
    THIRD_PARTY_LICENSES.txt \
    THIRD_PARTY_DEPENDENCIES.md >SHA256SUMS
)
"$ROOT/scripts/check-release-checksums.sh" "$fixture" >/dev/null

printf '%s\n' "windows binary" >"$fixture/aiah_0.1.1_windows_amd64.exe"
(
  cd "$fixture"
  sha256_write \
    aiah_0.1.1_linux_amd64 \
    aiah_0.1.1_windows_amd64.exe \
    LICENSE \
    NOTICE \
    THIRD_PARTY_LICENSES.txt \
    THIRD_PARTY_DEPENDENCIES.md >SHA256SUMS
)
if "$ROOT/scripts/check-release-checksums.sh" "$fixture" >/dev/null 2>&1; then
  echo "error: an unsupported Windows artifact was accepted" >&2
  exit 1
fi

echo "release checksum and Linux-only artifact guards: OK"
