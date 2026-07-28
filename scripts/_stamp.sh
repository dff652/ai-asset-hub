#!/usr/bin/env bash
# Shared build identity. Sourced by scripts/build.sh and scripts/release-build.sh.
#
# One copy on purpose: this change exists so a binary can say which build it is,
# and two copies of the stamping rules is exactly how a local build and a
# released build come to disagree about that.
#
# Exports VERSION, COMMIT, DATE and LDFLAGS. Callers may pre-set VERSION,
# COMMIT or DATE; SOURCE_DATE_EPOCH keeps DATE reproducible for build systems
# that provide it. EXTRA_LDFLAGS is prepended (release builds pass "-s -w").

VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
COMMIT="${COMMIT:-$(git rev-parse HEAD 2>/dev/null || echo '')}"
if [[ -n "${SOURCE_DATE_EPOCH:-}" ]]; then
  DATE="$(date -u -d "@$SOURCE_DATE_EPOCH" +%Y-%m-%dT%H:%M:%SZ)"
else
  DATE="${DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
fi

_stamp_pkg="github.com/dff652/ai-asset-hub/internal/version"
LDFLAGS="${EXTRA_LDFLAGS:-} -X ${_stamp_pkg}.Version=${VERSION} -X ${_stamp_pkg}.Commit=${COMMIT} -X ${_stamp_pkg}.Date=${DATE}"

export VERSION COMMIT DATE LDFLAGS
