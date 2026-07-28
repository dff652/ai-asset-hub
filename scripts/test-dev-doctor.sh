#!/usr/bin/env bash
# Isolated regression test: dev-doctor must never permit Go toolchain downloads.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fixture="$(mktemp -d)"
cleanup() {
  rm -rf "$fixture"
}
trap cleanup EXIT

fake_bin="$fixture/bin"
fake_goroot="$fixture/goroot"
go_log="$fixture/go.log"
mkdir -p "$fake_bin" "$fake_goroot/bin"

cat >"$fake_bin/go" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\t%s\n' "${GOTOOLCHAIN:-}" "$*" >>"$DOCTOR_GO_LOG"
case "$*" in
  version)
    echo "go version go1.26.5 linux/amd64"
    ;;
  "env GOROOT")
    echo "$DOCTOR_FAKE_GOROOT"
    ;;
  *)
    echo "unexpected fake go invocation: $*" >&2
    exit 2
    ;;
esac
EOF
cat >"$fake_bin/golangci-lint" <<'EOF'
#!/usr/bin/env bash
echo "golangci-lint has version 1.62.2 built with go1.26.5"
EOF
cat >"$fake_goroot/bin/gofmt" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
chmod +x "$fake_bin/go" "$fake_bin/golangci-lint" "$fake_goroot/bin/gofmt"
ln -s "$fake_goroot/bin/gofmt" "$fake_bin/gofmt"

PATH="$fake_bin:/usr/bin:/bin" \
  DOCTOR_GO_LOG="$go_log" \
  DOCTOR_FAKE_GOROOT="$fake_goroot" \
  "$ROOT/scripts/dev-doctor.sh" >/dev/null

if awk -F '\t' '$1 != "local" { exit 1 }' "$go_log"; then
  echo "dev-doctor local-toolchain guard: OK"
else
  echo "error: dev-doctor invoked go without GOTOOLCHAIN=local" >&2
  cat "$go_log" >&2
  exit 1
fi
