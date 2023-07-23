.PHONY: help
help: ## Display this help information
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
	  | sort | awk 'BEGIN {FS = ":.*?## "}; \
	  {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

clean: ## Clean the build directory
	rm -rf target

build-injector-linux: build-injector-linux-amd build-injector-linux-arm

build-injector-linux-amd:
	rustup target add x86_64-unknown-linux-musl
	@test -s x86_64-linux-musl-cross || curl https://musl.cc/x86_64-linux-musl-cross.tgz | tar -xz

	export PATH="$$PWD/x86_64-linux-musl-cross/bin:$$PATH"; \
	export CARGO_TARGET_X86_64_UNKOWN_LINUX_MUSL_LINKER=x86_64-linux-musl-cc; \
	cargo build --target x86_64-unknown-linux-musl --release; \
	x86_64-linux-musl-strip target/x86_64-unknown-linux-musl/release/zarf-injector

build-injector-linux-arm:
	rustup target add aarch64-unknown-linux-musl
	@test -s aarch64-linux-musl-cross || curl https://zarf-public.s3-us-gov-west-1.amazonaws.com/pipelines/aarch64-linux-musl-cross.tgz | tar -xz

	export PATH="$$PWD/aarch64-linux-musl-cross/bin:$$PATH"; \
	export CARGO_TARGET_AARCH64_UNKOWN_LINUX_MUSL_LINKER=aarch64-linux-musl-cc; \
	cargo build --target aarch64-unknown-linux-musl --release; \
	aarch64-linux-musl-strip target/aarch64-unknown-linux-musl/release/zarf-injector
