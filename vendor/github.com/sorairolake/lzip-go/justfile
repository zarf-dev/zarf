# SPDX-FileCopyrightText: 2024 Shun Sakai
#
# SPDX-License-Identifier: Apache-2.0 OR MIT

alias fmt := golangci-lint-fmt
alias lint := golangci-lint-run
alias build-cmd := build-cmd-debug

# Run default recipe
_default:
    just -l

# Remove generated artifacts
clean:
    go clean

# Run tests
test:
    go test ./...

# Run `golangci-lint`
golangci-lint: golangci-lint-fmt golangci-lint-run

# Run the formatter
golangci-lint-fmt:
    golangci-lint fmt

# Run the linter
golangci-lint-run:
    golangci-lint run

# Run `pkgsite`
pkgsite:
    pkgsite -http "0.0.0.0:8080"

# Build `glzip` command in debug mode
build-cmd-debug $CGO_ENABLED="0":
    go build ./cmd/glzip

# Build `glzip` command in release mode
build-cmd-release $CGO_ENABLED="0":
    go build -ldflags="-s -w" -trimpath ./cmd/glzip

# Build `glzip(1)`
build-man:
    asciidoctor -b manpage docs/man/man1/glzip.1.adoc

# Run the linter for GitHub Actions workflow files
lint-github-actions:
    actionlint -verbose

# Run the formatter for the README
fmt-readme:
    npx prettier -w README.md

# Increment the version
bump part:
    bump-my-version bump {{ part }}
