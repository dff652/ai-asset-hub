#!/usr/bin/env bash
# Portable SHA256 helpers for Linux (sha256sum) and macOS (shasum).

sha256_value() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
    return
  fi
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
    return
  fi
  echo "error: sha256sum or shasum is required" >&2
  return 1
}

sha256_write() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$@"
    return
  fi
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$@"
    return
  fi
  echo "error: sha256sum or shasum is required" >&2
  return 1
}

sha256_check_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum -c "$1"
    return
  fi
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 -c "$1"
    return
  fi
  echo "error: sha256sum or shasum is required" >&2
  return 1
}
