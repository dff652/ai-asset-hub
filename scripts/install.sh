#!/bin/sh
# Install a pinned aiah release after verifying its published SHA256.

set -eu

DEFAULT_AIAH_VERSION=0.1.11

die() {
  echo "error: $*" >&2
  exit 1
}

need() {
  command -v "$1" >/dev/null 2>&1 || die "$1 is required"
}

# Inlined rather than sourced from scripts/_sha256.sh. This script is published
# as `curl ... | sh`, where $0 is not a real path: resolving a helper "next to
# it" falls back to the current directory, so any file a local user drops there
# would be sourced, and the network fallback fetched the verifier itself over
# the very channel the verifier exists to check. A verifier that can be
# substituted verifies nothing, so it must not be loaded at runtime.
sha256_value() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
    return
  fi
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
    return
  fi
  die "sha256sum or shasum is required"
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
# The supported set mirrors scripts/_release_platforms.sh. It is repeated rather
# than sourced because this script is published as `curl ... | sh`, where there
# is no checkout to source anything from -- the same reason the checksum helper
# is inlined below.
case "$(uname -s)" in
  Linux) goos=linux ;;
  Darwin) goos=darwin ;;
  *) die "$(uname -s) is not supported; releases ship Linux and macOS only" ;;
esac
case "$(uname -m)" in
  x86_64 | amd64) goarch=amd64 ;;
  arm64 | aarch64) goarch=arm64 ;;
  *) die "$(uname -m) is not supported; releases ship amd64 and arm64 only" ;;
esac
# Linux arm64 compiles in CI but has no acceptance coverage, so no binary is
# published for it. Refusing here is better than a download that 404s.
if [ "$goos/$goarch" = "linux/arm64" ]; then
  die "Linux arm64 has no published binary; only linux/amd64, darwin/arm64 and darwin/amd64 ship"
fi

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

asset=aiah_${version}_${goos}_${goarch}
release_base=https://github.com/dff652/ai-asset-hub/releases/download/v$version
checksums=$tmp_dir/SHA256SUMS
binary=$tmp_dir/$asset

curl -fsSL "$release_base/SHA256SUMS" -o "$checksums"

# Checked before downloading the binary. A release predating this platform would
# otherwise fail as a bare curl 404, which says nothing about why.
expected_count=$(awk -v name="$asset" '$2 == name { count++ } END { print count + 0 }' \
  "$checksums")
if [ "$expected_count" -eq 0 ]; then
  die "v$version publishes no $goos/$goarch binary (looked for $asset); try a newer AIAH_VERSION"
fi
[ "$expected_count" -eq 1 ] ||
  die "SHA256SUMS must contain exactly one entry for $asset"

curl -fsSL "$release_base/$asset" -o "$binary"
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
