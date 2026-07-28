#!/usr/bin/env bash
# Fake HOME/project dry-run for the Phase 2 closed loop:
#   build → diff → apply → scan → doctor → rollback → scan
#
# Never touches your real $HOME. All writes go under a temp directory.
# Usage (from repo root):
#   ./scripts/demo-apply-scan-loop.sh
#   ./scripts/demo-apply-scan-loop.sh workspace-2b
#   KEEP_WORKDIR=1 ./scripts/demo-apply-scan-loop.sh   # leave temp dir for inspection

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

FIXTURE="${1:-workspace-valid}"
MANIFEST="$ROOT/testdata/$FIXTURE/manifest.yaml"
if [[ ! -f "$MANIFEST" ]]; then
  echo "error: manifest not found: $MANIFEST" >&2
  echo "usage: $0 [workspace-valid|workspace-2b]" >&2
  exit 2
fi

echo "==> building aiah"
"$ROOT/scripts/build.sh" >/dev/null
AIAH="$ROOT/build/aiah"

WORKDIR="${WORKDIR:-$(mktemp -d "${TMPDIR:-/tmp}/aiah-demo.XXXXXX")}"
DIST="$WORKDIR/dist"
FAKE_HOME="$WORKDIR/fake-home"
FAKE_PROJECT="$WORKDIR/fake-project"
mkdir -p "$DIST" "$FAKE_HOME" "$FAKE_PROJECT"

cleanup() {
  if [[ "${KEEP_WORKDIR:-}" == "1" ]]; then
    echo "==> KEEP_WORKDIR=1; left at $WORKDIR"
    return
  fi
  rm -rf "$WORKDIR"
}
trap cleanup EXIT

echo "==> workdir: $WORKDIR"
echo "==> fixture: $FIXTURE"

echo "==> validate"
"$AIAH" validate --manifest "$MANIFEST" --output json >/dev/null
echo "    ok"

echo "==> build"
BUILD_JSON="$WORKDIR/build.json"
"$AIAH" build \
  --manifest "$MANIFEST" \
  --profile personal \
  --out "$DIST" \
  --output json >"$BUILD_JSON"
ARCHIVE_NAME="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["package"]["archive"])' "$BUILD_JSON")"
PACKAGE="$DIST/$ARCHIVE_NAME"
if [[ ! -f "$PACKAGE" ]]; then
  echo "error: package missing: $PACKAGE" >&2
  exit 1
fi
echo "    package: $PACKAGE"

TARGETS="claude,codex"
if [[ "$FIXTURE" == "workspace-2b" ]]; then
  TARGETS="claude,codex,grok"
  export EXAMPLE_SERVICE_TOKEN="${EXAMPLE_SERVICE_TOKEN:-aiah-demo-secret}"
fi

echo "==> scan (before apply)"
"$AIAH" scan --home "$FAKE_HOME" --project "$FAKE_PROJECT" --output json \
  | python3 -c 'import json,sys; r=json.load(sys.stdin); print("    candidates=", r["summary"]["candidateAssets"])'

echo "==> diff (dry-run)"
"$AIAH" diff \
  --package "$PACKAGE" \
  --home "$FAKE_HOME" \
  --project "$FAKE_PROJECT" \
  --targets "$TARGETS" \
  --output json \
  | python3 -c 'import json,sys; r=json.load(sys.stdin); s=r["summary"]; print("    create=%s update=%s ok=%s" % (s["create"], s["update"], r["ok"]))'

echo "==> apply"
APPLY_JSON="$WORKDIR/apply.json"
"$AIAH" apply \
  --package "$PACKAGE" \
  --home "$FAKE_HOME" \
  --project "$FAKE_PROJECT" \
  --targets "$TARGETS" \
  --output json >"$APPLY_JSON"
python3 -c 'import json,sys; r=json.load(open(sys.argv[1])); print("    written=%s backupId=%s" % (r["summary"]["written"], r.get("backupId","")))' "$APPLY_JSON"

echo "==> scan (after apply)"
SCAN_AFTER="$WORKDIR/scan-after.json"
"$AIAH" scan --home "$FAKE_HOME" --project "$FAKE_PROJECT" --output json >"$SCAN_AFTER"
python3 -c 'import json,sys; r=json.load(open(sys.argv[1])); print("    candidates=%s byType=%s" % (r["summary"]["candidateAssets"], r["summary"].get("candidateByType",{})))' "$SCAN_AFTER"

echo "==> doctor"
DOCTOR_JSON="$WORKDIR/doctor.json"
"$AIAH" doctor --home "$FAKE_HOME" --project "$FAKE_PROJECT" --output json >"$DOCTOR_JSON"
python3 -c 'import json,sys; r=json.load(open(sys.argv[1])); s=r["summary"]; assert r["ok"] and s["unchanged"] > 0 and s["locallyModified"] == 0 and s["missing"] == 0, r; print("    ok=%s unchanged=%s backups=%s" % (r["ok"], s["unchanged"], s["backups"]))' "$DOCTOR_JSON"

echo "==> sample installed paths"
if [[ "$FIXTURE" == "workspace-valid" ]]; then
  ls -la "$FAKE_HOME/.claude/skills/shared-review/" 2>/dev/null || true
  ls -la "$FAKE_HOME/.agents/skills/shared-review/" 2>/dev/null || true
else
  ls -la "$FAKE_HOME/.grok/skills/review/" 2>/dev/null || true
  ls -la "$FAKE_PROJECT/CLAUDE.md" 2>/dev/null || true
fi

echo "==> rollback"
"$AIAH" rollback \
  --home "$FAKE_HOME" \
  --project "$FAKE_PROJECT" \
  --output json \
  | python3 -c 'import json,sys; r=json.load(sys.stdin); print("    ok=%s restored=%d removed=%d" % (r["ok"], len(r.get("restored",[])), len(r.get("removed",[]))))'

echo "==> scan (after rollback) — expect candidates == 0"
CAND="$("$AIAH" scan --home "$FAKE_HOME" --project "$FAKE_PROJECT" --output json \
  | python3 -c 'import json,sys; print(json.load(sys.stdin)["summary"]["candidateAssets"])')"
echo "    candidates= $CAND"
if [[ "$CAND" != "0" ]]; then
  echo "error: expected 0 candidates after rollback, got $CAND" >&2
  exit 1
fi

echo "==> OK: closed loop succeeded for $FIXTURE"
echo "    (real \$HOME was never used)"
