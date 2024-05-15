# SPDX-License-Identifier: Apache-2.0
# SPDX-FileCopyrightText: 2021-Present The Zarf Authors

.PHONY: help
help: ## Display this help information
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
	  | sort | awk 'BEGIN {FS = ":.*?## "}; \
	  {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

clean: ## Clean the build directory
	rm -rf target

cross-injector-linux: cross-injector-amd cross-injector-arm

cross-injector-amd: 
	rustup target add x86_64-unknown-linux-musl
	test -s x86_64-linux-musl-cross || curl https://zarf-public.s3-us-gov-west-1.amazonaws.com/pipelines/x86_64-linux-musl-cross.tgz | tar -xz
	export PATH="$$PWD/x86_64-linux-musl-cross/bin:$$PATH"
	export CARGO_TARGET_X86_64_UNKNOWN_LINUX_MUSL_LINKER=x86_64-linux-musl-cc

	cargo install cross --git https://github.com/cross-rs/cross

	cross build --target x86_64-unknown-linux-musl --release


cross-injector-arm:
	rustup target add aarch64-unknown-linux-musl
	test -s aarch64-linux-musl-cross || curl https://zarf-public.s3-us-gov-west-1.amazonaws.com/pipelines/aarch64-linux-musl-cross.tgz | tar -xz
	export PATH="$$PWD/aarch64-linux-musl-cross/bin:$$PATH"
	export CARGO_TARGET_AARCH64_UNKNOWN_LINUX_MUSL_LINKER=aarch64-linux-musl-cc

	cross build --target aarch64-unknown-linux-musl --release



build-injector-linux: build-injector-linux-amd build-injector-linux-arm ## Build the Zarf injector for AMD64 and ARM64

build-injector-linux-amd: ## Build the Zarf injector for AMD64
	rustup target add x86_64-unknown-linux-musl

	if [ "$(shell uname -m)" = "arm64" ] || [ "$(shell uname -m)" = "aarch64" ]; then \
		test -s x86_64-linux-musl-cross || curl https://zarf-public.s3-us-gov-west-1.amazonaws.com/pipelines/x86_64-linux-musl-cross.tgz | tar -xz; \
		export PATH="$$PWD/x86_64-linux-musl-cross/bin:$$PATH"; \
		export CARGO_TARGET_X86_64_UNKNOWN_LINUX_MUSL_LINKER=x86_64-linux-musl-cc; \
		cargo build --target x86_64-unknown-linux-musl --release; \
	elif [ "$(shell uname -m)" = "x86_64" ]; then \
		cargo build --target x86_64-unknown-linux-musl --release; \
	fi

build-injector-linux-arm: ## Build the Zarf injector for ARM64
	rustup target add aarch64-unknown-linux-musl

	if [ "$(shell uname -m)" = "arm64" ] || [ "$(shell uname -m)" = "aarch64" ]; then \
		cargo build --target aarch64-unknown-linux-musl --release; \
	elif [ "$(shell uname -m)" = "x86_64" ]; then \
		test -s aarch64-linux-musl-cross || curl https://zarf-public.s3-us-gov-west-1.amazonaws.com/pipelines/aarch64-linux-musl-cross.tgz | tar -xz; \
		export PATH="$$PWD/aarch64-linux-musl-cross/bin:$$PATH"; \
		export CARGO_TARGET_AARCH64_UNKNOWN_LINUX_MUSL_LINKER=aarch64-linux-musl-cc; \
		cargo build --target aarch64-unknown-linux-musl --release; \
	fi

list-sizes: ## List the sizes of the Zarf injector binaries
	@echo '\n\033[0;36mSize of Zarf injector binaries:\033[0m\n'; \
	du --si target/x86_64-unknown-linux-musl/release/zarf-injector; \
	du --si target/aarch64-unknown-linux-musl/release/zarf-injector

build-with-docker: ## Build the Zarf injector using Docker
	docker run --rm --user "$(id -u)":"$(id -g)" -v $$PWD:/usr/src/zarf-injector -w /usr/src/zarf-injector rust:1.71.0-bookworm make build-injector-linux
