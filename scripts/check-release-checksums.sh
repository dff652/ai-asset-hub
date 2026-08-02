#!/usr/bin/env bash
# Verify release artifacts with the SHA256 tool available on this host.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RELEASE_DIR="${1:-$ROOT/dist/release}"

if [[ ! -f "$RELEASE_DIR/SHA256SUMS" ]]; then
  echo "error: missing $RELEASE_DIR/SHA256SUMS" >&2
  exit 1
fi

# shellcheck source=scripts/_release_platforms.sh
source "$ROOT/scripts/_release_platforms.sh"

shopt -s nullglob
release_paths=("$RELEASE_DIR"/aiah_*)
shopt -u nullglob
if [[ "${#release_paths[@]}" -eq 0 ]]; then
  echo "error: $RELEASE_DIR contains no aiah_* binary" >&2
  exit 1
fi
# The version is read from what is present rather than passed in, so this also
# verifies a downloaded release, not just one built here.
release_version="${release_paths[0]##*/}"
release_version="${release_version#aiah_}"
release_version="${release_version%%_*}"

mapfile -t expected_binaries < <(release_binary_names "$release_version")
if [[ "${#release_paths[@]}" -ne "${#expected_binaries[@]}" ]]; then
  echo "error: release has ${#release_paths[@]} binaries, expected ${#expected_binaries[@]}" >&2
  printf '  present: %s\n' "${release_paths[@]##*/}" >&2
  printf '  expected: %s\n' "${expected_binaries[@]}" >&2
  exit 1
fi
for expected_binary in "${expected_binaries[@]}"; do
  if [[ ! -f "$RELEASE_DIR/$expected_binary" ]]; then
    echo "error: missing $expected_binary" >&2
    exit 1
  fi
  if ! awk -v required="$expected_binary" \
    '$2 == required { count++ } END { exit count != 1 }' \
    "$RELEASE_DIR/SHA256SUMS"; then
    echo "error: $expected_binary must have exactly one SHA256SUMS entry" >&2
    exit 1
  fi
done

required_files=(
  LICENSE
  NOTICE
  THIRD_PARTY_LICENSES.txt
  THIRD_PARTY_DEPENDENCIES.md
)
for required_file in "${required_files[@]}"; do
  if [[ ! -f "$RELEASE_DIR/$required_file" ]]; then
    echo "error: missing $RELEASE_DIR/$required_file" >&2
    exit 1
  fi
  if ! awk -v required="$required_file" '$2 == required { found = 1 } END { exit !found }' \
    "$RELEASE_DIR/SHA256SUMS"; then
    echo "error: $required_file is not covered by SHA256SUMS" >&2
    exit 1
  fi
done

# shellcheck source=scripts/_sha256.sh
source "$ROOT/scripts/_sha256.sh"
(
  cd "$RELEASE_DIR"
  sha256_check_file SHA256SUMS
)
