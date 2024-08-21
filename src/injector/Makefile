# SPDX-License-Identifier: Apache-2.0
# SPDX-FileCopyrightText: 2021-Present The Zarf Authors

.PHONY: help
help: ## Display this help information
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
	  | sort | awk 'BEGIN {FS = ":.*?## "}; \
	  {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

clean: ## Clean the build directory
	rm -rf target

# This is for local dev with mac arm machines since our cross images are not supported on that platform
build-injector-darwin-arm:
	rustup target add aarch64-unknown-linux-musl
	cargo build --target aarch64-unknown-linux-musl --release

# $HOME/.cargo/bin must be added to path for cross to work
install-cross:
	cargo install cross --git https://github.com/cross-rs/cross --tag v0.2.5

build-injector-linux: build-injector-linux-amd build-injector-linux-arm ## Build the Zarf injector for AMD64 and ARM64

build-injector-linux-amd: install-cross
	cross build --target aarch64-unknown-linux-musl --release

build-injector-linux-arm: install-cross
	cross build --target x86_64-unknown-linux-musl --release

list-sizes: ## List the sizes of the Zarf injector binaries
	@echo '\n\033[0;36mSize of Zarf injector binaries:\033[0m\n'; \
	du --si target/x86_64-unknown-linux-musl/release/zarf-injector; \
	du --si target/aarch64-unknown-linux-musl/release/zarf-injector
