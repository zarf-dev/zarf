# Repository Guidelines

## Project Structure & Module Organization

Zarf is a Go CLI (`main.go`, `go.mod`). Code is in `src/`: CLI wiring in `src/cmd`, APIs in `src/api`, shared types in `src/types`, libraries in `src/pkg`, and internals in `src/internal`. Keep unit tests beside the code they exercise as `*_test.go`; E2E and integration suites live in `src/test/`. Manifests and examples belong in `packages/` and `examples/`; the Astro docs site is in `site/`. Do not edit `vendor/` by hand.

## Build, Test, and Development Commands

- `make build` tidies and vendors modules, then builds the host CLI.
- `CGO_ENABLED=1 make test-unit` runs tests with the race detector and atomic coverage; `CGO_ENABLED=1 make test-unit-quick` skips both for a faster edit loop.
- `make lint-go` runs the configured `golangci-lint` checks.
- `make test-e2e-without-cluster` runs E2E journeys that need no cluster. Before `make test-e2e`, verify the selected cluster with `kubectl cluster-info`; it runs both suites and requires a reachable, configured cluster. For an isolated local target, use `kind create cluster --name zarf-e2e-$(id -un)` (then `kind delete cluster --name zarf-e2e-$(id -un)` when finished).
- `make docs-and-schema` regenerates CLI reference docs and `zarf.schema.json`. Run it after changing commands or schema types.
- In `site/`, use `npm ci`, then `npm run dev`, `npm run check`, or `npm run build`.

## Coding Style & Naming Conventions

Write idiomatic Go and let `golangci-lint fmt` apply `gofmt` and `goimports`; do not manually align imports. Use tabs as Go tooling emits them. Keep package and file names lowercase and descriptive; use `PascalCase` for exported identifiers and `camelCase` otherwise. New Go files need the repository SPDX header. Handle errors explicitly and use specific `//nolint:<linter>` comments only when justified.

Install hooks with `pre-commit install`; they enforce formatting, linting, credentials checks, and generated schema/docs consistency.

## Testing Guidelines

Add focused package-local tests for behavior changes, with `TestThing` / `TestThing_condition` names and table cases where useful. Use `testify` assertions consistently with nearby tests. Add or update a numbered `src/test/e2e/` journey for user-visible CLI behavior; avoid requiring a cluster unless needed.

## Commit & Pull Request Guidelines

Use signed, signed-off Conventional Commit messages, for example `fix(helm): preserve repository credentials` (`git commit -s -S`). Branch from `main`, open a draft PR early, link its issue (for example, `Fixes #123`), and explain user impact and testing. Apply `needs-adr`, `needs-docs`, and `needs-tests` labels when applicable; include regenerated artifacts and screenshots for UI/documentation changes. PRs must be releasable and pass automated checks.
