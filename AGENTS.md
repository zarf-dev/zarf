# Repository Guidelines

## Project Structure & Module Organization

Zarf is a Go CLI (`main.go`, `go.mod`). Application code under `src/`. Unit tests stay alongside code; E2E and integration suites live in `src/test/`. Production support packages in `packages/`, Examples in `examples/`; Astro docs are in `site/`. Do not edit `vendor/` by hand.

## Build, Test, and Development Commands

- `make build` tidies, vendors, and builds the CLI.
- `make test-unit` runs tests with race detection and atomic coverage; `make test-unit-quick` skips both.
- `make lint-go` runs configured `golangci-lint` checks.
- During development, run the focused journey for quick feedback: `go test ./src/test/e2e/ -v -run TestName -count=1 -failfast`. `make test-e2e` is permitted as a final check when reasonable.
- Cluster-dependent E2E tests need a reachable `kubectl` context; validate it with `kubectl cluster-info`. For isolation, create `kind create cluster --name zarf-e2e-$(id -un)` and delete it after testing. Some tests also require a Zarf-initialized cluster.
- `make docs-and-schema` regenerates CLI reference docs and `zarf.schema.json`. Run it after changing commands or schema types.
- In `site/`, use `npm ci`, then `npm run dev`, `npm run check`, or `npm run build`.

## Coding Style & Naming Conventions

Write idiomatic Go; `golangci-lint fmt` applies `gofmt` and `goimports`. Keep packages and files lowercase and descriptive; use `PascalCase` for exported identifiers and `camelCase` otherwise. New Go files need the SPDX header. Handle errors and justify every `//nolint:<linter>` comment.

Install hooks with `pre-commit install`; they enforce formatting, linting, credentials checks, and generated schema/docs consistency.

## Testing Guidelines

Add package-local tests for behavior changes, with `TestThing` / `TestThing_condition` names and table cases where useful. Use `testify` assertions. Add or update a numbered `src/test/e2e/` journey for user-visible CLI behavior; avoid cluster requirements unless needed.

## Version Compatibility

Preserve package compatibility in both directions: packages created by older CLIs must deploy with newer CLIs, and packages created by newer CLIs should deploy with older CLIs. Exceptions require a `VersionRequirement` minimum version or a documented breaking change; include its impact and migration guidance in the handoff.

## Human Handoff

Humans own commits, issues, and pull requests. Agents may prepare changes and handoff material, but must not author or create them. Give the human author factual context on the change, user impact, tests, and follow-up; it informs their own response, not copy-and-paste submission text.
