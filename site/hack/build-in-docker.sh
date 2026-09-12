#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# SPDX-FileCopyrightText: 2021-Present The Zarf Authors
#
# Builds or serves the docs site inside a container, so contributors do not need
# a local Node install. The Node version comes from netlify.toml's NODE_VERSION,
# which keeps a local build on the same toolchain Netlify uses.
#
# Usage:
#   site/hack/build-in-docker.sh [command]
#
# Commands:
#   build      (default) npm run build -> site/dist. Latest docs only, and what
#              you want for day-to-day work.
#   versions   npm run build:versions -> site/dist. Full Netlify parity: Latest
#              at the root plus every archived release under /<slug>/. Clones
#              tags from GitHub, so it needs network and is much slower.
#   check      npm run check (astro check, no output emitted).
#   install    npm ci only.
#   dev        astro dev with live reload on $PORT. No build step needed.
#   preview    astro preview, serving an existing site/dist on $PORT.
#   serve      netlify-cli serve, which also applies netlify.toml redirects.
#   shell      interactive shell in the build container.
#
# Environment:
#   RUNTIME        docker (default) or podman.
#   NODE_VERSION   overrides the version parsed from netlify.toml.
#   PORT           port for dev/preview/serve (default 4321, 8888 for serve).
#   IMAGE          overrides the container image entirely.
#
# Prerequisites:
#   - docker or podman

set -euo pipefail

SCRIPT_NAME="$(basename "${BASH_SOURCE[0]}")"
SITE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REPO_ROOT="$(cd "${SITE_DIR}/.." && pwd)"
NETLIFY_TOML="${REPO_ROOT}/netlify.toml"

CMD="${1:-build}"
RUNTIME="${RUNTIME:-docker}"

usage() {
    sed -n '/^# Usage:/,/^# Prerequisites:/p' "${BASH_SOURCE[0]}" | sed 's/^#[[:space:]]\{0,1\}//'
}

if [ "${CMD}" = "-h" ] || [ "${CMD}" = "--help" ] || [ "${CMD}" = "help" ]; then
    usage
    exit 0
fi

if ! command -v "${RUNTIME}" >/dev/null 2>&1; then
    echo "${SCRIPT_NAME}: ${RUNTIME} not found; install it or set RUNTIME=podman" >&2
    exit 1
fi

# netlify.toml is the source of truth for the Node version. site/package.json's
# "engines" range is deliberately not consulted: npm only warns on a mismatch
# and Netlify ignores the field, so following it here would build on a different
# Node than production does.
if [ -z "${NODE_VERSION:-}" ]; then
    NODE_VERSION="$(sed -n 's/^[[:space:]]*NODE_VERSION[[:space:]]*=[[:space:]]*"\([^"]*\)".*/\1/p' "${NETLIFY_TOML}" | head -n1)"
fi
if [ -z "${NODE_VERSION}" ]; then
    echo "${SCRIPT_NAME}: could not parse NODE_VERSION from ${NETLIFY_TOML}; set NODE_VERSION explicitly" >&2
    exit 1
fi

# hack/build-versions.mjs shells out to git for its throwaway worktrees, and the
# slim image ships no git. Everything else stays on slim.
case "${CMD}" in
    versions | shell) DEFAULT_IMAGE="node:${NODE_VERSION}" ;;
    *) DEFAULT_IMAGE="node:${NODE_VERSION}-slim" ;;
esac
IMAGE="${IMAGE:-${DEFAULT_IMAGE}}"

case "${CMD}" in
    serve) PORT="${PORT:-8888}" ;;
    *) PORT="${PORT:-4321}" ;;
esac

# The repo root is the mount, not site/, because the prebuild step copies
# schemas out of ../src/pkg/schema and examples out of ../examples.
run_args=(
    --rm
    -v "${REPO_ROOT}:/repo"
    -w /repo/site
    -e HOME=/tmp
    -e npm_config_cache=/tmp/.npm
)

# Write node_modules and dist as the invoking user instead of root. Rootless
# podman already maps the host user into the container, where --user would
# re-map it to the wrong id, so keep-id is the equivalent there.
if [ "${RUNTIME}" = "podman" ]; then
    run_args+=(--userns=keep-id --security-opt label=disable)
else
    run_args+=(--user "$(id -u):$(id -g)")
fi

if [ -t 0 ] && [ -t 1 ]; then
    run_args+=(-it)
fi

case "${CMD}" in
    dev | preview | serve) run_args+=(-p "${PORT}:${PORT}") ;;
esac

run_in_container() {
    "${RUNTIME}" run "${run_args[@]}" "${IMAGE}" "$@"
}

install_deps() {
    echo "==> installing dependencies (node ${NODE_VERSION})"
    run_in_container npm ci --no-audit --no-fund
}

ensure_deps() {
    if [ ! -d "${SITE_DIR}/node_modules" ]; then
        install_deps
    fi
}

require_dist() {
    if [ ! -d "${SITE_DIR}/dist" ]; then
        echo "${SCRIPT_NAME}: site/dist not found; run '${SCRIPT_NAME} build' first" >&2
        exit 1
    fi
}

case "${CMD}" in
    install)
        install_deps
        ;;
    build)
        ensure_deps
        echo "==> building Latest docs -> site/dist"
        run_in_container npm run build
        ;;
    versions)
        ensure_deps
        echo "==> building Latest plus archived versions -> site/dist"
        run_in_container npm run build:versions
        ;;
    check)
        ensure_deps
        run_in_container npm run check
        ;;
    dev)
        ensure_deps
        echo "==> dev server on http://localhost:${PORT}"
        run_in_container npx astro dev --host 0.0.0.0 --port "${PORT}"
        ;;
    preview)
        ensure_deps
        require_dist
        echo "==> preview on http://localhost:${PORT}"
        run_in_container npx astro preview --host 0.0.0.0 --port "${PORT}"
        ;;
    serve)
        require_dist
        # netlify-cli reads netlify.toml from the repo root, so this is the only
        # mode that exercises the redirect rules; astro preview ignores them.
        echo "==> netlify serve on http://localhost:${PORT} (redirects applied)"
        "${RUNTIME}" run "${run_args[@]}" -w /repo "${IMAGE}" \
            npx --yes netlify-cli serve --dir site/dist --port "${PORT}"
        ;;
    shell)
        run_in_container bash
        ;;
    *)
        echo "${SCRIPT_NAME}: unknown command '${CMD}'" >&2
        usage >&2
        exit 1
        ;;
esac
