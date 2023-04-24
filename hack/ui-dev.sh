#! /usr/bin/env sh
# This script is called during `npm run dev`
# It is used to start the Sveltekit frontend + Go Chi backend
# It also injects the current CLI dev version similar to how the Makefile does

CLI_VERSION=$(git describe --tags --always)
BUILD_ARGS="-s -w -X 'github.com/defenseunicorns/zarf/src/config.CLIVersion=$CLI_VERSION'"

API_DEV_PORT=5173 \
    API_PORT=3333 \
    API_TOKEN=insecure \
    concurrently --names "ui,api" \
    -c "gray.bold,yellow" \
    "vite dev" \
    "nodemon -e go -x 'go run -ldflags=\"$BUILD_ARGS\" ../../main.go dev ui -l=trace || exit 1'"
