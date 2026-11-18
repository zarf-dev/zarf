# GGUF Parser

A Go library and CLI (`cmd/gguf-parser`) that parses
[GGUF](https://github.com/ggml-org/ggml/blob/master/docs/gguf.md) model files — local, remote via
ranged HTTP reads, or pulled from the HuggingFace, ModelScope, or Ollama registries — and estimates
their memory usage and maximum tokens per second without downloading entire files. Estimates track
llama.cpp's allocation behavior; deviation from actual usage is typically around 100MiB.

## Project Structure

- Root package `github.com/gpustack/gguf-parser-go` (package `gguf_parser`) — the library.
  - `file*.go` — parsing (`file.go`, `file_option.go`, `file_from_remote.go`, `file_from_distro.go`), metadata (`file_metadata.go`), architecture detection (`file_architecture.go`), tokenizer (`file_tokenizer.go`), and the estimators.
  - `ggml.go` / `scalar.go` — GGUF types and scalar values.
  - `file_estimate__llamacpp.go` / `file_estimate__stablediffusioncpp.go` — per-backend estimators.
  - `file_estimate_option.go` — estimator options (offload layers, context size, flash attention, mmap, adapters, RPC).
  - `cache.go` — ranged-read caching.
  - `ollama_*.go` — Ollama registry readers.
  - `gen.go` / `gen.*.go` + `zz_generated.*.go` — code generation entry points and generated output.
- `cmd/gguf-parser/` — the CLI; its own `go.mod`, built on urfave/cli/v2, dot-importing the root package.
- `util/` — helper packages: `anyx`, `bytex`, `funcx`, `httpx`, `json`, `osx`, `ptr`, `signalx`, `slicex`, `stringx`.
- `Makefile` — the whole workflow is Makefile-driven; there is no `hack/` directory.
- `.github/workflows/` — CI.
- `.sbin/` — git-ignored downloaded tools (goimports-reviser, golangci-lint, lipo).

## Architecture

The parse pipeline reads the GGUF header, metadata, and tensor list, then detects the architecture
(`file_architecture.go`), builds the tokenizer (`file_tokenizer.go`), and feeds per-backend
estimators (`file_estimate__llamacpp.go` for llama.cpp, `file_estimate__stablediffusioncpp.go` for
stable-diffusion.cpp), which project RAM/VRAM usage and maximum tokens per second from that
metadata. Remote files are read through ranged HTTP requests backed by `cache.go`, so only the
bytes needed for parsing are fetched. Registry readers (`file_from_distro.go`, `ollama_*.go`)
resolve HuggingFace, ModelScope, and Ollama model references into parseable sources.

## Finding sources

- `README.md` owns the estimation semantics and CLI usage; read it before changing an estimator or
  a flag. Its examples and option tables change together with `cmd/gguf-parser/main.go`.
- `.github/copilot-instructions.md` owns the code-review rules and links back here for the conventions, so
  each fact is stated once.

## Development

- `make deps` — tidy and download modules for both Go modules (root library and `cmd/gguf-parser`); `DEPS_UPDATE=true` upgrades them.
- `make generate` — run the `//go:generate` stages (stringer + regression) in both modules, producing `zz_generated.*.go`.
- `make lint` — goimports-reviser (import order std/general/company/project) plus `golangci-lint run --fix`, in both modules; tools are downloaded into `.sbin/`.
- `make test` — `go test -v -failfast -race -cover` over the root module.
- `make benchmark` — run `Benchmark*` with `-benchmem`.
- `make build` (`make gguf-parser`) — cross-compile the CLI into `.dist/` for darwin/linux/windows.
- `make package` — docker buildx image build (set `PACKAGE_PUBLISH=true` to push).
- `make ci` — deps, generate, lint, test, build (the default target).
- Model-dependent tests read the model from `TEST_MODEL_PATH` and skip when it is unset: `TEST_MODEL_PATH=/path/to/model.gguf make test`.

Hard invariants:

- Editing types or enums that feed the generators (GGML types, quantizations, architectures)
  requires regenerated `zz_generated.*` files (`make generate`). Never hand-edit `zz_generated.*`
  or `gen.*` files.
- Both Go modules must build and pass checks. `make ci` covers the root library and
  `cmd/gguf-parser` together; do not leave one module behind.
- Parsing must never panic on malformed input; return an error. Every declared length, count, or
  offset in a GGUF file is untrusted — validate it against the remaining file size before
  allocating, seeking, or slicing.
- Estimates must stay aligned with llama.cpp's allocation behavior. Cite an upstream reference or
  add test coverage for an estimation change.
- Commit messages follow Conventional Commits (`type: subject`).

## Go conventions

- Prefer clarity over cleverness to simplify long-term code maintenance.
- Run lint checks locally whenever modifying Go source code.
- Handle errors explicitly; never use panics for control flow.
- Keep interfaces minimal; accept abstractions, return concrete implementations.
- Use concise names accurately reflecting purpose and domain meaning.
- Name multi-word Go source files in snake_case (`file_from_remote.go`, `file_estimate__llamacpp.go`), never flat-concatenated.
- Write focused functions performing one responsibility and nothing else.
- Prefer composition and values over inheritance-like design patterns.
- Keep concurrency simple, safe, justified, and minimally applied — the parser leans on mmap and
  chunked reads, so minimize mutable shared state that risks data races.
- Document exported APIs with behavior, expectations, and constraints; doc comments end in periods (godot).
- Use `any`, not `interface{}` (enforced by gofmt rewrite rules).
- Keep comments plain and short: no emoji, no decorative symbols. State the point in words.
- Respect the linter config: gofumpt with extra rules, 150-character line limit, importas aliasing, import order std/general/company/project.

## Testing conventions

- Use testify for assertions.
- Prefer table-driven cases with a shared execution loop where the shape fits.
- Verify exactly one behavior or contract per test case; keep cases declarative.
- Build fixtures through helpers for consistency and maintainability.
- Assert observable final state instead of implementation details.
- Keep tests deterministic and `-race` clean; the suite runs with `-race -failfast`.
- Fail immediately when setup errors invalidate test assumptions.
- Model-dependent tests skip via `t.Skip` when `TEST_MODEL_PATH` is unset; they must never fail for that.
