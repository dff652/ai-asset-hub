#!/usr/bin/env bash
# Verify release artifacts with the SHA256 tool available on this host.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RELEASE_DIR="${1:-$ROOT/dist/release}"

if [[ ! -f "$RELEASE_DIR/SHA256SUMS" ]]; then
  echo "error: missing $RELEASE_DIR/SHA256SUMS" >&2
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
