#!/usr/bin/env bash
# The platforms a release actually ships, shared by the builder and the verifier
# so the two cannot disagree about what a complete release looks like.
#
# Membership is decided by acceptance coverage, not by what compiles: linux/amd64
# and darwin/arm64 each run the full suite on their own CI runner, and
# darwin/amd64 rides along because a pure-Go CGO_ENABLED=0 binary differs only in
# codegen across architectures of one OS. Everything else stays a CI build-health
# check, because shipping a binary reads as a support claim.

RELEASE_PLATFORMS=(
  linux/amd64
  darwin/arm64
  darwin/amd64
)

# release_binary_names <version> -> one asset name per shipped platform.
release_binary_names() {
  local version=$1 platform
  for platform in "${RELEASE_PLATFORMS[@]}"; do
    printf '%s\n' "aiah_${version}_${platform%/*}_${platform#*/}"
  done
}
