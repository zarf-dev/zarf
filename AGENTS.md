# Repository Guidelines

## Project Structure & Module Organization

Zarf is a Go CLI (`main.go`, `go.mod`). Under `src/`, `cmd` contains CLI wiring, `api` APIs, `types` domain types, `pkg` libraries, and `internal` implementation. Unit tests stay alongside code; E2E and integration suites live in `src/test/`. Manifests and examples are in `packages/` and `examples/`; Astro docs are in `site/`. Do not edit `vendor/` by hand.

## Build, Test, and Development Commands

- `make build` tidies, vendors, and builds the CLI.
- `CGO_ENABLED=1 make test-unit` runs tests with race detection and atomic coverage; `CGO_ENABLED=1 make test-unit-quick` skips both.
- `make lint-go` runs the configured `golangci-lint` checks.
- `make test-e2e-without-cluster` runs E2E journeys that need no cluster. Before `make test-e2e`, verify the selected cluster with `kubectl cluster-info`; it runs both suites and requires a reachable cluster. For an isolated local target, use `kind create cluster --name zarf-e2e-$(id -un)` (then `kind delete cluster --name zarf-e2e-$(id -un)`).
- `make docs-and-schema` regenerates CLI reference docs and `zarf.schema.json`. Run it after changing commands or schema types.
- In `site/`, use `npm ci`, then `npm run dev`, `npm run check`, or `npm run build`.

## Coding Style & Naming Conventions

Write idiomatic Go; `golangci-lint fmt` applies `gofmt` and `goimports`. Use tabs. Keep package and file names lowercase and descriptive; use `PascalCase` for exported identifiers and `camelCase` otherwise. New Go files need the SPDX header. Handle errors explicitly and justify every `//nolint:<linter>` comment.

Install hooks with `pre-commit install`; they enforce formatting, linting, credentials checks, and generated schema/docs consistency.

## Testing Guidelines

Add package-local tests for behavior changes, with `TestThing` / `TestThing_condition` names and table cases where useful. Use `testify` assertions consistently. Add or update a numbered `src/test/e2e/` journey for user-visible CLI behavior; avoid cluster requirements unless needed.

## Version Compatibility

Preserve package compatibility in both directions: packages created by older CLIs must deploy with newer CLIs, and packages created by newer CLIs should deploy with older CLIs. Exceptions require a `VersionRequirement` minimum version or a documented breaking change; state its impact and migration guidance in the PR.

## Commit & Pull Request Guidelines

Use signed, signed-off Conventional Commit messages, for example `fix(helm): preserve repository credentials` (`git commit -s -S`). Branch from `main`, open a draft PR early, link its issue (for example, `Fixes #123`), and explain impact and testing. Apply `needs-adr`, `needs-docs`, and `needs-tests` when applicable; include regenerated artifacts and UI/documentation screenshots. PRs must be releasable and pass automated checks.
