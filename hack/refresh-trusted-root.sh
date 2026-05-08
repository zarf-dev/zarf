#!/usr/bin/env bash
# Refresh the embedded Sigstore TrustedRoot used for keyless verification.
# Run before each release. Commit the result.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
EMBED_PATH="${REPO_ROOT}/src/pkg/utils/embedded_trusted_root.json"
ZARF_BIN="${REPO_ROOT}/build/zarf"

if [ ! -x "${ZARF_BIN}" ]; then
    echo "build/zarf not found; run 'make build' first" >&2
    exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
    echo "jq is required to format the embedded trusted root for reviewable diffs" >&2
    exit 1
fi

"${ZARF_BIN}" tools trusted-root create --with-default-services --out "${EMBED_PATH}"
jq --indent 2 . "${EMBED_PATH}" > "${EMBED_PATH}.tmp" && mv "${EMBED_PATH}.tmp" "${EMBED_PATH}"
echo "Refreshed ${EMBED_PATH}"
