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

# Keep the adversarial input internally coherent: every shipped binary is present
# and listed, the decoy exists and its checksum is valid. Only the exact required
# LICENSE entry is absent, so a pass here can only mean the decoy was accepted in
# its place -- not that some earlier check happened to fire first.
printf '%s\n' "decoy" >"$fixture/LICENSE.decoy"
printf '%s\n' "linux binary" >"$fixture/aiah_0.1.1_linux_amd64"
printf '%s\n' "darwin arm64 binary" >"$fixture/aiah_0.1.1_darwin_arm64"
printf '%s\n' "darwin amd64 binary" >"$fixture/aiah_0.1.1_darwin_amd64"
# shellcheck source=scripts/_sha256.sh
source "$ROOT/scripts/_sha256.sh"
(
  cd "$fixture"
  sha256_write \
    aiah_0.1.1_linux_amd64 \
    aiah_0.1.1_darwin_arm64 \
    aiah_0.1.1_darwin_amd64 \
    LICENSE.decoy \
    NOTICE \
    THIRD_PARTY_LICENSES.txt \
    THIRD_PARTY_DEPENDENCIES.md >SHA256SUMS
)

if "$ROOT/scripts/check-release-checksums.sh" "$fixture" >/dev/null 2>&1; then
  echo "error: LICENSE.decoy was accepted as the LICENSE checksum entry" >&2
  exit 1
fi

# A complete release passes, while adding an unvalidated platform artifact makes
# the same release invalid even when its checksum is correct. Shipping a binary
# reads as a support claim, so the shape of the set is checked, not just that
# every file present is intact.
(
  cd "$fixture"
  sha256_write \
    aiah_0.1.1_linux_amd64 \
    aiah_0.1.1_darwin_arm64 \
    aiah_0.1.1_darwin_amd64 \
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
    aiah_0.1.1_darwin_arm64 \
    aiah_0.1.1_darwin_amd64 \
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

# A release missing one of the platforms it claims to ship is incomplete, which
# the old "exactly one binary" rule could not express.
rm -f "$fixture/aiah_0.1.1_windows_amd64.exe" "$fixture/aiah_0.1.1_darwin_amd64"
(
  cd "$fixture"
  sha256_write \
    aiah_0.1.1_linux_amd64 \
    aiah_0.1.1_darwin_arm64 \
    LICENSE \
    NOTICE \
    THIRD_PARTY_LICENSES.txt \
    THIRD_PARTY_DEPENDENCIES.md >SHA256SUMS
)
if "$ROOT/scripts/check-release-checksums.sh" "$fixture" >/dev/null 2>&1; then
  echo "error: a release missing darwin/amd64 was accepted" >&2
  exit 1
fi


# Right count, wrong members. The count check alone would pass this, so it is the
# case that proves the per-platform lookup is load-bearing rather than redundant:
# a build that produced the wrong platform set is exactly this shape.
rm -f "$fixture"/aiah_0.1.1_*
printf '%s\n' "linux binary" >"$fixture/aiah_0.1.1_linux_amd64"
printf '%s\n' "darwin arm64 binary" >"$fixture/aiah_0.1.1_darwin_arm64"
printf '%s\n' "windows binary" >"$fixture/aiah_0.1.1_windows_amd64.exe"
(
  cd "$fixture"
  sha256_write \
    aiah_0.1.1_linux_amd64 \
    aiah_0.1.1_darwin_arm64 \
    aiah_0.1.1_windows_amd64.exe \
    LICENSE \
    NOTICE \
    THIRD_PARTY_LICENSES.txt \
    THIRD_PARTY_DEPENDENCIES.md >SHA256SUMS
)
if "$ROOT/scripts/check-release-checksums.sh" "$fixture" >/dev/null 2>&1; then
  echo "error: a release with the right count but the wrong platforms was accepted" >&2
  exit 1
fi


# Count matches and every expected name is listed, but one listed binary is not
# on disk. Without the per-file lookup this reaches the final sha256 -c and fails
# as a checksum error, which describes the symptom rather than the cause.
rm -f "$fixture"/aiah_0.1.1_*
printf '%s\n' "linux binary" >"$fixture/aiah_0.1.1_linux_amd64"
printf '%s\n' "darwin arm64 binary" >"$fixture/aiah_0.1.1_darwin_arm64"
printf '%s\n' "windows binary" >"$fixture/aiah_0.1.1_windows_amd64.exe"
(
  cd "$fixture"
  sha256_write \
    aiah_0.1.1_linux_amd64 \
    aiah_0.1.1_darwin_arm64 \
    aiah_0.1.1_windows_amd64.exe \
    LICENSE \
    NOTICE \
    THIRD_PARTY_LICENSES.txt \
    THIRD_PARTY_DEPENDENCIES.md >SHA256SUMS
  # Claim a checksum for a binary that was never built.
  printf '%s  %s\n' "$(printf 'ghost' | sha256_stdin)" aiah_0.1.1_darwin_amd64 >>SHA256SUMS
)
missing_output=$("$ROOT/scripts/check-release-checksums.sh" "$fixture" 2>&1) &&
  fail_missing=1
case "${missing_output:-}" in
  *"missing aiah_0.1.1_darwin_amd64"*) ;;
  *) echo "error: unclear message for a listed-but-absent binary: $missing_output" >&2; exit 1 ;;
esac
[ -z "${fail_missing:-}" ] || { echo "error: a listed-but-absent binary was accepted" >&2; exit 1; }

# A duplicated entry is ambiguous: two checksums for one name means the file that
# gets verified depends on awk's iteration, not on the release.
rm -f "$fixture"/aiah_0.1.1_*
printf '%s\n' "linux binary" >"$fixture/aiah_0.1.1_linux_amd64"
printf '%s\n' "darwin arm64 binary" >"$fixture/aiah_0.1.1_darwin_arm64"
printf '%s\n' "darwin amd64 binary" >"$fixture/aiah_0.1.1_darwin_amd64"
(
  cd "$fixture"
  sha256_write \
    aiah_0.1.1_linux_amd64 \
    aiah_0.1.1_darwin_arm64 \
    aiah_0.1.1_darwin_amd64 \
    aiah_0.1.1_darwin_amd64 \
    LICENSE \
    NOTICE \
    THIRD_PARTY_LICENSES.txt \
    THIRD_PARTY_DEPENDENCIES.md >SHA256SUMS
)
if "$ROOT/scripts/check-release-checksums.sh" "$fixture" >/dev/null 2>&1; then
  echo "error: a duplicated SHA256SUMS entry was accepted" >&2
  exit 1
fi

echo "check-release-checksums.sh: OK"
