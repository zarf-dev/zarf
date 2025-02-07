# SPDX-License-Identifier: Apache-2.0
# SPDX-FileCopyrightText: 2021-Present The Zarf Authors

.PHONY: help
help: ## Display this help information
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
	  | sort | awk 'BEGIN {FS = ":.*?## "}; \
	  {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

clean: ## Clean the build directory
	rm -rf target

build-injector-linux: cross-injector-amd cross-injector-arm

cross-injector-amd:
	rustup target add x86_64-unknown-linux-musl
	cargo install cross --git https://github.com/cross-rs/cross
	cross build --target x86_64-unknown-linux-musl --release


cross-injector-arm:
	rustup target add aarch64-unknown-linux-musl
	export CARGO_TARGET_AARCH64_UNKNOWN_LINUX_MUSL_LINKER=aarch64-linux-musl-cc
	cross build --target aarch64-unknown-linux-musl --release

list-sizes: ## List the sizes of the Zarf injector binaries
	@echo '\n\033[0;36mSize of Zarf injector binaries:\033[0m\n'; \
	du -k target/x86_64-unknown-linux-musl/release/zarf-injector; \
	du -k target/aarch64-unknown-linux-musl/release/zarf-injector
