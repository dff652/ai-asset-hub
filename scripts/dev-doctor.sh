#!/usr/bin/env bash
# Read-only development environment diagnostics.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

expected_go="$(awk '$1 == "toolchain" { print $2; exit }' go.mod)"
expected_lint="1.62.2"
failures=0

ok() {
  echo "ok: $*"
}

fail() {
  echo "error: $*" >&2
  failures=$((failures + 1))
}

for command_name in git python3 curl tar file; do
  if command -v "$command_name" >/dev/null 2>&1; then
    ok "$command_name = $(command -v "$command_name")"
  else
    fail "missing required command: $command_name"
  fi
done
if command -v sha256sum >/dev/null 2>&1; then
  ok "SHA256 tool = $(command -v sha256sum)"
elif command -v shasum >/dev/null 2>&1; then
  ok "SHA256 tool = $(command -v shasum)"
else
  fail "missing required command: sha256sum or shasum"
fi

if command -v go >/dev/null 2>&1; then
  # A doctor must not install what it is trying to diagnose. In a module with a
  # newer toolchain directive, the default GOTOOLCHAIN=auto can download Go
  # before `go version` returns, so every probe is pinned to the bundled binary.
  actual_go="$(GOTOOLCHAIN=local go version 2>/dev/null || true)"
  if [[ "$actual_go" == "go version $expected_go "* ]]; then
    ok "$actual_go"
  else
    fail "expected $expected_go, got: ${actual_go:-unavailable}"
  fi

  go_root="$(GOTOOLCHAIN=local go env GOROOT 2>/dev/null || true)"
  actual_gofmt=""
  expected_gofmt=""
  if command -v gofmt >/dev/null 2>&1 && command -v python3 >/dev/null 2>&1 &&
    [[ -n "$go_root" ]]; then
    actual_gofmt="$(python3 -c 'import os,sys; print(os.path.realpath(sys.argv[1]))' \
      "$(command -v gofmt)")"
    expected_gofmt="$(python3 -c 'import os,sys; print(os.path.realpath(sys.argv[1]))' \
      "$go_root/bin/gofmt")"
  fi
  if [[ -n "$actual_gofmt" && "$actual_gofmt" == "$expected_gofmt" ]]; then
    ok "gofmt matches selected GOROOT"
  else
    fail "gofmt does not match the selected Go toolchain"
  fi

  ok "GOTOOLCHAIN=local for read-only doctor probes"
else
  fail "go is not in PATH"
  fail "gofmt cannot be checked without go"
fi

if command -v golangci-lint >/dev/null 2>&1; then
  actual_lint="$(golangci-lint version 2>/dev/null || true)"
  if [[ "$actual_lint" == *"version $expected_lint "* ||
    "$actual_lint" == *"version v$expected_lint "* ]]; then
    ok "golangci-lint $expected_lint"
  else
    fail "expected golangci-lint $expected_lint, got: ${actual_lint:-unavailable}"
  fi
else
  fail "golangci-lint is not in PATH"
fi

if ((failures > 0)); then
  echo >&2
  echo "development environment is not ready ($failures failure(s))" >&2
  echo "Run ./scripts/bootstrap-dev.sh for the pinned user-local toolchain." >&2
  exit 1
fi

echo "development environment ready"
