#!/usr/bin/env bash
# Verify release artifacts with the SHA256 tool available on this host.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RELEASE_DIR="${1:-$ROOT/dist/release}"

if [[ ! -f "$RELEASE_DIR/SHA256SUMS" ]]; then
  echo "error: missing $RELEASE_DIR/SHA256SUMS" >&2
  exit 1
fi

shopt -s nullglob
release_paths=("$RELEASE_DIR"/aiah_*)
shopt -u nullglob
if [[ "${#release_paths[@]}" -ne 1 ]] || [[ ! -f "${release_paths[0]:-}" ]]; then
  echo "error: release must contain exactly one Linux amd64 binary" >&2
  exit 1
fi
release_binary="${release_paths[0]##*/}"
if [[ ! "$release_binary" =~ ^aiah_[0-9A-Za-z.+-]+_linux_amd64$ ]]; then
  echo "error: release must contain exactly one Linux amd64 binary" >&2
  exit 1
fi
if ! awk -v required="$release_binary" \
  '$2 == required { count++ } END { exit count != 1 }' \
  "$RELEASE_DIR/SHA256SUMS"; then
  echo "error: $release_binary must have exactly one SHA256SUMS entry" >&2
  exit 1
fi

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
