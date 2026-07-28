#!/usr/bin/env bash
# Generate or verify the verbatim license bundle shipped with release binaries.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

mode="${1:-generate}"
if [[ "$mode" != "generate" && "$mode" != "--check" ]]; then
  echo "usage: $0 [--check]" >&2
  exit 2
fi

destination="$ROOT/THIRD_PARTY_LICENSES.txt"
generated="$(mktemp "${TMPDIR:-/tmp}/aiah-licenses.XXXXXX")"
modules="$(mktemp "${TMPDIR:-/tmp}/aiah-modules.XXXXXX")"
cleanup() {
  rm -f "$generated" "$modules"
}
trap cleanup EXIT

GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
  go list -deps -f \
  '{{with .Module}}{{if not .Main}}{{.Path}}{{"\t"}}{{.Version}}{{"\t"}}{{.Dir}}{{end}}{{end}}' \
  ./cmd/aiah |
  awk 'NF' | sort -u >"$modules"

{
  echo "AI Asset Hub - Third-Party License Texts"
  echo
  echo "This file is generated from modules linked into the supported Linux amd64"
  echo "release binary."
  echo

  first_license=1
  while IFS=$'\t' read -r module version directory; do
    license_files=()
    for name in LICENSE LICENSE.txt LICENSE.md COPYING COPYING.txt NOTICE NOTICE.txt; do
      if [[ -f "$directory/$name" ]]; then
        license_files+=("$directory/$name")
      fi
    done

    override="$ROOT/docs/licenses/overrides/${module//\//_}@${version}.LICENSE"
    if ((${#license_files[@]} == 0)) && [[ -f "$override" ]]; then
      license_files+=("$override")
    fi
    if ((${#license_files[@]} == 0)); then
      echo "error: no license material found for $module@$version" >&2
      exit 1
    fi

    for license_file in "${license_files[@]}"; do
      if ((first_license == 0)); then
        echo
      fi
      echo "================================================================================"
      echo "Module: $module"
      echo "Version: $version"
      echo "File: $(basename "$license_file")"
      echo "================================================================================"
      cat "$license_file"
      first_license=0
    done
  done <"$modules"
} >"$generated"

if [[ "$mode" == "--check" ]]; then
  if [[ ! -f "$destination" ]] || ! cmp -s "$destination" "$generated"; then
    echo "error: THIRD_PARTY_LICENSES.txt is stale" >&2
    echo "Run ./scripts/generate-third-party-licenses.sh and review the result." >&2
    exit 1
  fi
  echo "third-party license bundle: OK"
else
  mv "$generated" "$destination"
  echo "generated $destination"
fi
