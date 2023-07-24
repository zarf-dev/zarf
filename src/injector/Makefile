# SPDX-License-Identifier: Apache-2.0
# SPDX-FileCopyrightText: 2021-Present The Zarf Authors

.PHONY: help
help: ## Display this help information
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
	  | sort | awk 'BEGIN {FS = ":.*?## "}; \
	  {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

clean: ## Clean the build directory
	rm -rf target

build-injector-linux: build-injector-linux-amd build-injector-linux-arm ## Build the Zarf injector for AMD64 and ARM64

build-injector-linux-amd: ## Build the Zarf injector for AMD64
	rustup target add x86_64-unknown-linux-musl

	if [ "$(shell uname -m)" = "arm64" ] || [ "$(shell uname -m)" = "aarch64" ]; then \
		test -s x86_64-linux-musl-cross || curl https://zarf-public.s3-us-gov-west-1.amazonaws.com/pipelines/x86_64-linux-musl-cross.tgz | tar -xz; \
		export PATH="$$PWD/x86_64-linux-musl-cross/bin:$$PATH"; \
		export CARGO_TARGET_X86_64_UNKNOWN_LINUX_MUSL_LINKER=x86_64-linux-musl-cc; \
		cargo build --target x86_64-unknown-linux-musl --release; \
		x86_64-linux-musl-strip target/x86_64-unknown-linux-musl/release/zarf-injector; \
	elif [ "$(shell uname -m)" = "x86_64" ]; then \
		cargo build --target x86_64-unknown-linux-musl --release; \
        strip target/x86_64-unknown-linux-musl/release/zarf-injector; \
	fi

build-injector-linux-arm: ## Build the Zarf injector for ARM64
	rustup target add aarch64-unknown-linux-musl

	if [ "$(shell uname -m)" = "arm64" ] || [ "$(shell uname -m)" = "aarch64" ]; then \
		cargo build --target aarch64-unknown-linux-musl --release; \
        strip target/aarch64-unknown-linux-musl/release/zarf-injector; \
	elif [ "$(shell uname -m)" = "x86_64" ]; then \
		test -s aarch64-linux-musl-cross || curl https://zarf-public.s3-us-gov-west-1.amazonaws.com/pipelines/aarch64-linux-musl-cross.tgz | tar -xz; \
		export PATH="$$PWD/aarch64-linux-musl-cross/bin:$$PATH"; \
		export CARGO_TARGET_AARCH64_UNKNOWN_LINUX_MUSL_LINKER=aarch64-linux-musl-cc; \
		cargo build --target aarch64-unknown-linux-musl --release; \
		aarch64-linux-musl-strip target/aarch64-unknown-linux-musl/release/zarf-injector; \
	fi
