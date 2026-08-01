#!/usr/bin/env bash
# Network-free regression tests for install.sh.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
INSTALLER="${INSTALLER:-$ROOT/scripts/install.sh}"
TEST_ROOT="$(mktemp -d)"
REAL_PATH="$PATH"
unset AIAH_VERSION
EXPECTED_DEFAULT_AIAH_VERSION=0.1.10

cleanup() {
  rm -rf "$TEST_ROOT"
}
trap cleanup EXIT

fail() {
  echo "error: $*" >&2
  exit 1
}

assert_same() {
  cmp -s "$1" "$2" || fail "$1 and $2 differ"
}

make_fake_tools() {
  local bin_dir=$1
  mkdir -p "$bin_dir"

  cat >"$bin_dir/uname" <<'EOF'
#!/bin/sh
case "$1" in
  -s) printf '%s\n' "$AIAH_TEST_UNAME_S" ;;
  -m) printf '%s\n' "$AIAH_TEST_UNAME_M" ;;
  *) exit 2 ;;
esac
EOF

  cat >"$bin_dir/curl" <<'EOF'
#!/bin/sh
set -eu
out=
url=
while [ "$#" -gt 0 ]; do
  case "$1" in
    -o)
      out=$2
      shift 2
      ;;
    -*)
      shift
      ;;
    *)
      url=$1
      shift
      ;;
  esac
done
[ -n "$out" ] && [ -n "$url" ]
printf '%s\n' "$url" >>"$AIAH_TEST_CURL_LOG"
case "$url" in
  */SHA256SUMS) cp "$AIAH_TEST_CHECKSUMS" "$out" ;;
  */_sha256.sh) cp "$AIAH_TEST_POISONED_HELPER" "$out" ;;
  */aiah_*) cp "$AIAH_TEST_BINARY" "$out" ;;
  *) exit 22 ;;
esac
EOF

  chmod +x "$bin_dir/uname" "$bin_dir/curl"
}

make_binary() {
  local path=$1
  local version=$2
  cat >"$path" <<EOF
#!/bin/sh
if [ "\${1:-}" = version ]; then
  echo "aiah $version, commit test"
  exit 0
fi
exit 2
EOF
  chmod +x "$path"
}

write_checksums() {
  local binary=$1
  local asset=$2
  local output=$3
  local digest
  digest=$(sha256sum "$binary" | awk '{print $1}')
  printf '%s  %s\n' "$digest" "$asset" >"$output"
}

run_installer() {
  local fake_bin=$1
  local install_dir=$2
  AIAH_INSTALL_DIR="$install_dir" \
    AIAH_TEST_UNAME_S="${AIAH_TEST_UNAME_S:-Linux}" \
    AIAH_TEST_UNAME_M="${AIAH_TEST_UNAME_M:-x86_64}" \
    PATH="$fake_bin:$REAL_PATH" \
    sh "$INSTALLER"
}

run_installer_piped() {
  local fake_bin=$1
  local install_dir=$2
  AIAH_INSTALL_DIR="$install_dir" \
    AIAH_TEST_UNAME_S="${AIAH_TEST_UNAME_S:-Linux}" \
    AIAH_TEST_UNAME_M="${AIAH_TEST_UNAME_M:-x86_64}" \
    PATH="$fake_bin:$REAL_PATH" \
    sh <"$INSTALLER"
}

fixture=$TEST_ROOT/fixture
fake_bin=$TEST_ROOT/fake-bin
mkdir -p "$fixture"
make_fake_tools "$fake_bin"
make_binary "$fixture/binary" "$EXPECTED_DEFAULT_AIAH_VERSION"
write_checksums "$fixture/binary" \
  "aiah_${EXPECTED_DEFAULT_AIAH_VERSION}_linux_amd64" "$fixture/SHA256SUMS"
export AIAH_TEST_BINARY=$fixture/binary
export AIAH_TEST_CHECKSUMS=$fixture/SHA256SUMS
# Behaves correctly on purpose: a verifier that still verifies is the hardest
# case, because only the fact that it was loaded at all proves the compromise.
poisoned_helper=$fixture/poisoned_sha256.sh
cat >"$poisoned_helper" <<'EOF'
sha256_value() { sha256sum "$1" | awk '{print $1}'; }
touch "$AIAH_TEST_HOSTILE_MARKER"
EOF
export AIAH_TEST_POISONED_HELPER=$poisoned_helper
export AIAH_TEST_HOSTILE_MARKER=$TEST_ROOT/hostile-executed
export AIAH_TEST_CURL_LOG=$TEST_ROOT/curl.log
: >"$AIAH_TEST_CURL_LOG"

# Successful verified install.
install_dir=$TEST_ROOT/success/bin
run_installer "$fake_bin" "$install_dir" >/dev/null
assert_same "$fixture/binary" "$install_dir/aiah"
[ -x "$install_dir/aiah" ] || fail "installed binary is not executable"
if find "$install_dir" -maxdepth 1 -name '.aiah.install.*' | grep -q .; then
  fail "successful install left a stage file"
fi

# Same version is a zero-download no-op.
: >"$AIAH_TEST_CURL_LOG"
before=$(sha256sum "$install_dir/aiah")
run_installer "$fake_bin" "$install_dir" >/dev/null
after=$(sha256sum "$install_dir/aiah")
[ "$before" = "$after" ] || fail "idempotent install changed the binary"
[ ! -s "$AIAH_TEST_CURL_LOG" ] || fail "idempotent install performed a download"

# A checksum mismatch must preserve the existing binary.
mismatch_dir=$TEST_ROOT/mismatch/bin
mkdir -p "$mismatch_dir"
make_binary "$mismatch_dir/aiah" 0.1.1
cp "$mismatch_dir/aiah" "$TEST_ROOT/old-mismatch"
printf '%064d  %s\n' 0 \
  "aiah_${EXPECTED_DEFAULT_AIAH_VERSION}_linux_amd64" >"$fixture/SHA256SUMS"
: >"$AIAH_TEST_CURL_LOG"
if run_installer "$fake_bin" "$mismatch_dir" >/dev/null 2>&1; then
  fail "checksum mismatch was accepted"
fi
assert_same "$TEST_ROOT/old-mismatch" "$mismatch_dir/aiah"

# Duplicate exact checksum entries are ambiguous and must be rejected.
write_checksums "$fixture/binary" \
  "aiah_${EXPECTED_DEFAULT_AIAH_VERSION}_linux_amd64" "$fixture/SHA256SUMS"
checksum_line=$(cat "$fixture/SHA256SUMS")
printf '%s\n%s\n' "$checksum_line" "$checksum_line" >"$fixture/SHA256SUMS"
if run_installer "$fake_bin" "$mismatch_dir" >/dev/null 2>&1; then
  fail "duplicate checksum entries were accepted"
fi
assert_same "$TEST_ROOT/old-mismatch" "$mismatch_dir/aiah"

# A failed final rename must preserve the existing binary and clean the stage.
write_checksums "$fixture/binary" \
  "aiah_${EXPECTED_DEFAULT_AIAH_VERSION}_linux_amd64" "$fixture/SHA256SUMS"
atomic_dir=$TEST_ROOT/atomic/bin
mkdir -p "$atomic_dir"
make_binary "$atomic_dir/aiah" 0.1.1
cp "$atomic_dir/aiah" "$TEST_ROOT/old-atomic"
fail_bin=$TEST_ROOT/fail-bin
mkdir -p "$fail_bin"
cat >"$fail_bin/mv" <<'EOF'
#!/bin/sh
exit 1
EOF
chmod +x "$fail_bin/mv"
if run_installer "$fail_bin:$fake_bin" "$atomic_dir" >/dev/null 2>&1; then
  fail "failed final rename was reported as success"
fi
assert_same "$TEST_ROOT/old-atomic" "$atomic_dir/aiah"
if find "$atomic_dir" -maxdepth 1 -name '.aiah.install.*' | grep -q .; then
  fail "failed install left a stage file"
fi

# Unsupported systems and architectures fail before downloading. Darwin and
# arm64 are explicit regression cases because both were previously published.
: >"$AIAH_TEST_CURL_LOG"
AIAH_TEST_UNAME_S=Darwin
export AIAH_TEST_UNAME_S
if run_installer "$fake_bin" "$TEST_ROOT/unsupported-os" >/dev/null 2>&1; then
  fail "unsupported operating system was accepted"
fi
[ ! -s "$AIAH_TEST_CURL_LOG" ] || fail "unsupported operating system downloaded files"
unset AIAH_TEST_UNAME_S

AIAH_TEST_UNAME_M=aarch64
export AIAH_TEST_UNAME_M
if run_installer "$fake_bin" "$TEST_ROOT/unsupported-arch" >/dev/null 2>&1; then
  fail "unsupported architecture was accepted"
fi
[ ! -s "$AIAH_TEST_CURL_LOG" ] || fail "unsupported architecture downloaded files"
unset AIAH_TEST_UNAME_M

# The verifier must never be loaded at runtime. install.sh is published as
# `curl ... | sh`, where $0 is not a real path: a sourced helper would resolve
# to the current directory, and the old network fallback fetched the verifier
# over the very channel it exists to check.
: >"$AIAH_TEST_CURL_LOG"
install_dir=$TEST_ROOT/no-helper-fetch/bin
run_installer_piped "$fake_bin" "$install_dir" >/dev/null
if grep -q '_sha256\.sh' "$AIAH_TEST_CURL_LOG"; then
  fail "install.sh fetched its own checksum verifier over the network"
fi
[ ! -e "$AIAH_TEST_HOSTILE_MARKER" ] ||
  fail "a downloaded checksum verifier was executed"
assert_same "$fixture/binary" "$install_dir/aiah"

# A hostile _sha256.sh in the working directory must have no effect. Before the
# fix this file was sourced, which both ran arbitrary code as the installing
# user and let sha256_value return whatever the attacker chose.
hostile=$TEST_ROOT/hostile
mkdir -p "$hostile"
cat >"$hostile/_sha256.sh" <<'EOF'
sha256_value() { sha256sum "$1" | awk '{print $1}'; }
touch "$AIAH_TEST_HOSTILE_MARKER"
EOF
install_dir=$TEST_ROOT/hostile-cwd/bin
( cd "$hostile" && run_installer_piped "$fake_bin" "$install_dir" >/dev/null )
[ ! -e "$AIAH_TEST_HOSTILE_MARKER" ] ||
  fail "a _sha256.sh in the working directory was executed"
assert_same "$fixture/binary" "$install_dir/aiah"

# A masking helper returns exactly what install.sh expects, so a sourced
# verifier would let any binary through without a visible failure.
cat >"$hostile/_sha256.sh" <<'EOF'
sha256_value() { echo "$AIAH_TEST_MASK_DIGEST"; }
EOF
export AIAH_TEST_MASK_DIGEST=0000000000000000000000000000000000000000000000000000000000000000
printf '%s  %s\n' \
  "0000000000000000000000000000000000000000000000000000000000000000" \
  "aiah_${EXPECTED_DEFAULT_AIAH_VERSION}_linux_amd64" >"$fixture/SHA256SUMS"
install_dir=$TEST_ROOT/hostile-mismatch/bin
if ( cd "$hostile" && run_installer_piped "$fake_bin" "$install_dir" >/dev/null 2>&1 ); then
  fail "a working-directory helper masked a checksum mismatch"
fi
[ ! -e "$install_dir/aiah" ] || fail "masked mismatch still installed a binary"
write_checksums "$fixture/binary" \
  "aiah_${EXPECTED_DEFAULT_AIAH_VERSION}_linux_amd64" "$fixture/SHA256SUMS"

sh_default=$(awk -F= '$1 == "DEFAULT_AIAH_VERSION" { print $2 }' \
  "$ROOT/scripts/install.sh")
[ "$sh_default" = "$EXPECTED_DEFAULT_AIAH_VERSION" ] ||
  fail "install.sh default version is $sh_default, want $EXPECTED_DEFAULT_AIAH_VERSION"

echo "install.sh: OK"
