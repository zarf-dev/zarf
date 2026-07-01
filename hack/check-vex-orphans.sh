#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# SPDX-FileCopyrightText: 2021-Present The Zarf Authors
#
# Checks for orphaned VEX statements: entries in .vex/zarf.openvex.json whose
# vulnerability ID no longer appears in the current grype scan results
# (build/grype.json). Orphaned statements should be reviewed and removed so the
# VEX document stays accurate and does not silently suppress future findings with
# the same ID.
#
# This script always exits 0 (advisory warnings only); the nightly CVE workflow
# captures its output as part of the artifact report.
#
# Usage:
#   hack/check-vex-orphans.sh
#
# Prerequisites:
#   - jq installed
#   - build/grype.json exists (run: make scan-grype)

set -euo pipefail

VEX_FILE=".vex/zarf.openvex.json"
GRYPE_JSON="build/grype.json"

if [[ ! -f "$VEX_FILE" ]]; then
  echo "VEX file not found: $VEX_FILE" >&2
  exit 1
fi

if [[ ! -f "$GRYPE_JSON" ]]; then
  echo "Grype JSON not found: $GRYPE_JSON — run 'make scan-grype' first" >&2
  exit 1
fi

# Collect all vulnerability IDs seen by grype in the latest scan:
#   - .matches[].vulnerability.id           → active findings
#   - .ignoredMatches[].match.vulnerability.id → findings suppressed by VEX or ignore rules
# Together these represent the full set of CVEs the scanner is aware of.
GRYPE_IDS=$(jq -r '
  [
    (.matches // [] | .[].vulnerability.id),
    (.ignoredMatches // [] | .[].vulnerability.id)
  ] | flatten | unique | sort | .[]
' "$GRYPE_JSON")

# Collect the vulnerability name from every VEX statement.
VEX_IDS=$(jq -r '.statements // [] | .[].vulnerability.name' "$VEX_FILE")

if [[ -z "$VEX_IDS" ]]; then
  echo "✓ No VEX statements found — nothing to check"
  exit 0
fi

ORPHAN_COUNT=0

while IFS= read -r vex_id; do
  [[ -z "$vex_id" ]] && continue
  if ! echo "$GRYPE_IDS" | grep -qxF "$vex_id"; then
    echo "⚠ Orphaned VEX statement: $vex_id"
    echo "  This vulnerability no longer appears in the current grype scan."
    echo "  If it has been patched or the dependency removed, delete this statement from $VEX_FILE."
    ORPHAN_COUNT=$((ORPHAN_COUNT + 1))
  fi
done <<< "$VEX_IDS"

if [[ $ORPHAN_COUNT -gt 0 ]]; then
  echo ""
  echo "Found $ORPHAN_COUNT orphaned VEX statement(s). Review and remove stale entries from $VEX_FILE."
else
  echo "✓ All VEX statements match current grype findings"
fi

# Always exit 0 — orphan detection is advisory; the nightly run captures this
# output in the uploaded artifact but does not fail on orphans.
exit 0
