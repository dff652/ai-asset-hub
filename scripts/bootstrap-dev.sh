#!/usr/bin/env bash
# Install the repository-pinned Go toolchain and golangci-lint without root.
#
# This is an explicit, per-device bootstrap. It never edits shell profiles,
# never replaces a system Go installation, and refuses to overwrite an
# unrelated file in the user-local bin directory.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
# shellcheck source=scripts/_sha256.sh
source "$ROOT/scripts/_sha256.sh"

toolchain="$(awk '$1 == "toolchain" { sub(/^go/, "", $2); print $2; exit }' go.mod)"
if [[ -z "$toolchain" ]]; then
  echo "error: go.mod has no toolchain directive" >&2
  exit 1
fi

case "$(uname -s)/$(uname -m)" in
  Linux/x86_64)
    platform="linux-amd64"
    expected_sha256="5c2c3b16caefa1d968a94c1daca04a7ca301a496d9b086e17ad77bb81393f053"
    ;;
  Linux/aarch64 | Linux/arm64)
    platform="linux-arm64"
    expected_sha256="fe4789e92b1f33358680864bbe8704289e7bb5fc207d80623c308935bd696d49"
    ;;
  Darwin/x86_64)
    platform="darwin-amd64"
    expected_sha256="6231d8d3b8f5552ec6cbf6d685bdd5482e1e703214b120e89b3bf0d7bf1ef725"
    ;;
  Darwin/arm64)
    platform="darwin-arm64"
    expected_sha256="efb87ff28af9a188d0536ef5d42e63dd52ba8263cd7344a993cc48dd11dedb6a"
    ;;
  *)
    echo "error: unsupported development platform: $(uname -s)/$(uname -m)" >&2
    echo "Linux and macOS on amd64/arm64 are supported by this bootstrap." >&2
    exit 1
    ;;
esac

case "$toolchain" in
  1.26.5) ;;
  *)
    echo "error: bootstrap checksums do not cover go$toolchain" >&2
    echo "Update and review scripts/bootstrap-dev.sh with the official archive checksums." >&2
    exit 1
    ;;
esac

data_root="${XDG_DATA_HOME:-${HOME}/.local/share}"
bin_root="${XDG_BIN_HOME:-${HOME}/.local/bin}"
install_root="$data_root/aiah/toolchains/go$toolchain"
archive="go${toolchain}.${platform}.tar.gz"

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "error: required bootstrap command is missing: $1" >&2
    exit 1
  fi
}

for command_name in awk curl git mktemp tar; do
  require_command "$command_name"
done
if ! command -v sha256sum >/dev/null 2>&1 && ! command -v shasum >/dev/null 2>&1; then
  echo "error: required bootstrap command is missing: sha256sum or shasum" >&2
  exit 1
fi

current_go=""
if command -v go >/dev/null 2>&1; then
  current_go="$(GOTOOLCHAIN=local go version 2>/dev/null || true)"
fi

if [[ "$current_go" == "go version go${toolchain} "* ]]; then
  go_binary="$(GOTOOLCHAIN=local go env GOROOT)/bin/go"
  echo "==> Go $toolchain already available at $go_binary"
else
  if [[ ! -x "$install_root/bin/go" ]]; then
    if [[ -e "$install_root" ]]; then
      echo "error: refusing to replace incomplete toolchain directory: $install_root" >&2
      exit 1
    fi
    mkdir -p "$(dirname "$install_root")"
    workdir="$(mktemp -d "$(dirname "$install_root")/.bootstrap.XXXXXX")"
    cleanup() {
      rm -rf "$workdir"
    }
    trap cleanup EXIT

    echo "==> downloading $archive"
    curl --proto '=https' --tlsv1.2 --fail --silent --show-error --location \
      "https://go.dev/dl/$archive" \
      --output "$workdir/$archive"

    actual_sha256="$(sha256_value "$workdir/$archive")"
    if [[ "$actual_sha256" != "$expected_sha256" ]]; then
      echo "error: SHA256 mismatch for $archive" >&2
      exit 1
    fi
    echo "$archive: OK"

    tar -xzf "$workdir/$archive" -C "$workdir"
    if [[ ! -x "$workdir/go/bin/go" ]]; then
      echo "error: verified archive did not contain go/bin/go" >&2
      exit 1
    fi
    extracted_version="$(GOTOOLCHAIN=local "$workdir/go/bin/go" version)"
    if [[ "$extracted_version" != "go version go${toolchain} "* ]]; then
      echo "error: archive reports an unexpected version: $extracted_version" >&2
      exit 1
    fi
    mv "$workdir/go" "$install_root"
  fi
  go_binary="$install_root/bin/go"
  echo "==> Go $toolchain available at $go_binary"
fi

mkdir -p "$bin_root"
for tool_name in go gofmt; do
  link_path="$bin_root/$tool_name"
  target_path="$(dirname "$go_binary")/$tool_name"
  if [[ -e "$link_path" || -L "$link_path" ]]; then
    if [[ "$(readlink "$link_path" 2>/dev/null || true)" != "$target_path" ]]; then
      echo "error: refusing to replace existing $link_path" >&2
      exit 1
    fi
  else
    ln -s "$target_path" "$link_path"
  fi
done

export PATH="$bin_root:$PATH"

lint_version="1.62.2"
current_lint=""
if command -v golangci-lint >/dev/null 2>&1; then
  current_lint="$(golangci-lint version 2>/dev/null || true)"
fi
if [[ "$current_lint" == *"version $lint_version "* ||
  "$current_lint" == *"version v$lint_version "* ]]; then
  echo "==> golangci-lint $lint_version already available"
else
  lint_path="$bin_root/golangci-lint"
  if [[ -e "$lint_path" || -L "$lint_path" ]]; then
    echo "error: refusing to replace existing $lint_path" >&2
    exit 1
  fi
  echo "==> installing golangci-lint $lint_version"
  GOBIN="$bin_root" GOTOOLCHAIN=local "$go_binary" install \
    "github.com/golangci/golangci-lint/cmd/golangci-lint@v$lint_version"
fi

echo "==> development toolchain installed"
echo "    PATH must contain: $bin_root"
"$ROOT/scripts/dev-doctor.sh"
