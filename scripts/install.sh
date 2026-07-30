#!/bin/sh
# Install a pinned aiah release after verifying its published SHA256.

set -eu

DEFAULT_AIAH_VERSION=0.1.6

die() {
  echo "error: $*" >&2
  exit 1
}

need() {
  command -v "$1" >/dev/null 2>&1 || die "$1 is required"
}

installed_version() {
  "$1" version 2>/dev/null |
    awk 'NR == 1 && $1 == "aiah" { sub(/,$/, "", $2); print $2 }'
}

if [ "$#" -ne 0 ]; then
  die "install.sh accepts configuration through AIAH_VERSION and AIAH_INSTALL_DIR"
fi

version=${AIAH_VERSION:-$DEFAULT_AIAH_VERSION}
version=${version#v}
case "$version" in
  '' | *[!0-9A-Za-z.+-]*) die "invalid AIAH_VERSION: $version" ;;
esac

if [ "${AIAH_INSTALL_DIR+x}" = x ]; then
  install_dir=$AIAH_INSTALL_DIR
else
  : "${HOME:?HOME is required when AIAH_INSTALL_DIR is unset}"
  install_dir=$HOME/.local/bin
fi
case "$install_dir" in
  '~')
    : "${HOME:?HOME is required to expand AIAH_INSTALL_DIR}"
    install_dir=$HOME
    ;;
  '~/'*)
    : "${HOME:?HOME is required to expand AIAH_INSTALL_DIR}"
    install_dir=$HOME/${install_dir#\~/}
    ;;
esac
[ -n "$install_dir" ] || die "AIAH_INSTALL_DIR must not be empty"

need uname
case "$(uname -s)" in
  Linux) ;;
  *) die "only Linux amd64 is currently supported" ;;
esac
case "$(uname -m)" in
  x86_64 | amd64) ;;
  *) die "only Linux amd64 is currently supported" ;;
esac

target=$install_dir/aiah
if [ -x "$target" ] && [ "$(installed_version "$target")" = "$version" ]; then
  echo "aiah $version is already installed at $target"
  exit 0
fi

need curl
need mktemp
need awk

tmp_dir=$(mktemp -d)
stage=
cleanup() {
  if [ -n "$stage" ]; then
    rm -f "$stage"
  fi
  rm -rf "$tmp_dir"
}
trap cleanup EXIT HUP INT TERM

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" 2>/dev/null && pwd || true)
if [ -n "$script_dir" ] && [ -f "$script_dir/_sha256.sh" ]; then
  sha_helper=$script_dir/_sha256.sh
else
  sha_helper=$tmp_dir/_sha256.sh
  curl -fsSL \
    "https://raw.githubusercontent.com/dff652/ai-asset-hub/v$version/scripts/_sha256.sh" \
    -o "$sha_helper"
fi
# shellcheck source=scripts/_sha256.sh
. "$sha_helper"

asset=aiah_${version}_linux_amd64
release_base=https://github.com/dff652/ai-asset-hub/releases/download/v$version
checksums=$tmp_dir/SHA256SUMS
binary=$tmp_dir/$asset

curl -fsSL "$release_base/SHA256SUMS" -o "$checksums"
curl -fsSL "$release_base/$asset" -o "$binary"

expected_count=$(awk -v name="$asset" '$2 == name { count++ } END { print count + 0 }' \
  "$checksums")
[ "$expected_count" -eq 1 ] ||
  die "SHA256SUMS must contain exactly one entry for $asset"
expected=$(awk -v name="$asset" '$2 == name { print $1 }' "$checksums")
actual=$(sha256_value "$binary")
if [ "$actual" != "$expected" ]; then
  die "SHA256 verification failed for $asset"
fi

chmod 0755 "$binary"
if [ "$(installed_version "$binary")" != "$version" ]; then
  die "$asset did not report version $version"
fi

mkdir -p "$install_dir"
stage=$(mktemp "$install_dir/.aiah.install.XXXXXX")
cp "$binary" "$stage"
chmod 0755 "$stage"
mv "$stage" "$target"
stage=

echo "installed aiah $version to $target"
case ":${PATH:-}:" in
  *":$install_dir:"*) ;;
  *) echo "hint: add $install_dir to PATH" ;;
esac
