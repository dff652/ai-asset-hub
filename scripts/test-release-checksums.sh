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
# shellcheck source=scripts/_sha256.sh
source "$ROOT/scripts/_sha256.sh"
(
  cd "$fixture"
  sha256_write \
    LICENSE.decoy \
    NOTICE \
    THIRD_PARTY_LICENSES.txt \
    THIRD_PARTY_DEPENDENCIES.md >SHA256SUMS
)

if "$ROOT/scripts/check-release-checksums.sh" "$fixture" >/dev/null 2>&1; then
  echo "error: LICENSE.decoy was accepted as the LICENSE checksum entry" >&2
  exit 1
fi
echo "release checksum exact-name guard: OK"
