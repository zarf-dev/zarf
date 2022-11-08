# Provide a default value for the operating system architecture used in tests, e.g. " APPLIANCE_MODE=true|false make test-e2e ARCH=arm64"
ARCH ?= amd64
# The image tag used for the zarf agent, defaults to a dev image tag
AGENT_IMAGE ?= dev-agent:e32f41ab50f994302614adf62ab6f13a7ecfbb25
# The zarf injector registry binary to use
INJECTOR_VERSION := v0.22.35-9g1fc3

######################################################################################

# Figure out which Zarf binary we should use based on the operating system we are on
ZARF_BIN := ./build/zarf
ifeq ($(OS),Windows_NT)
	ZARF_BIN := $(addsuffix .exe,$(ZARF_BIN))
else
	UNAME_S := $(shell uname -s)
	UNAME_P := $(shell uname -p)
	ifneq ($(UNAME_S),Linux)
		ifeq ($(UNAME_S),Darwin)
			ZARF_BIN := $(addsuffix -mac,$(ZARF_BIN))
		endif
		ifeq ($(UNAME_P),i386)
			ZARF_BIN := $(addsuffix -intel,$(ZARF_BIN))
		endif
		ifeq ($(UNAME_P),arm)
			ZARF_BIN := $(addsuffix -apple,$(ZARF_BIN))
		endif
	endif
endif

CLI_VERSION := $(if $(shell git describe --tags),$(shell git describe --tags),"UnknownVersion")
BUILD_ARGS := -s -w -X 'github.com/defenseunicorns/zarf/src/config.CLIVersion=$(CLI_VERSION)'
.DEFAULT_GOAL := help

.PHONY: help
help: ## Show a list of all targets
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
	| sed -n 's/^\(.*\): \(.*\)##\(.*\)/\1:\3/p' \
	| column -t -s ":"

vm-init: ## Make a vagrant VM (usage -> make vm-init OS=ubuntu)
	vagrant destroy -f
	vagrant up --no-color ${OS}
	echo -e "\n\n\n\033[1;93m  ✅ BUILD COMPLETE.  To access this environment, run \"vagrant ssh ${OS}\"\n\n\n"

vm-destroy: ## Destroy the vagrant VM
	vagrant destroy -f

clean: ## Clean the build directory
	rm -rf build

destroy: ## Run `zarf destroy` on the current cluster
	$(ZARF_BIN) destroy --confirm --remove-components
	rm -fr build

remove-packages: ## Remove all zarf packages recursively
	find . -type f -name 'zarf-package-*' -delete

ensure-ui-build-dir:
	mkdir -p build/ui
	touch build/ui/index.html

check-ui: ## Build the Zarf UI if needed
	@ if [ ! -z "$(shell command -v shasum)" ]; then\
	    if test "$(shell ./.hooks/print-ui-diff.sh | shasum)" != "$(shell cat build/ui/git-info.txt | shasum)" ; then\
		    $(MAKE) build-ui;\
		    ./.hooks/print-ui-diff.sh > build/ui/git-info.txt;\
	    fi;\
	else\
        $(MAKE) build-ui;\
	fi

build-ui: ## Build the Zarf UI
	npm ci
	npm run build

build-cli-linux-amd: check-ui ## Build the Zarf CLI for Linux on AMD64
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="$(BUILD_ARGS)" -o build/zarf main.go

build-cli-linux-arm: check-ui ## Build the Zarf CLI for Linux on ARM
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="$(BUILD_ARGS)" -o build/zarf-arm main.go

build-cli-mac-intel: check-ui ## Build the Zarf CLI for macOS on AMD64
	GOOS=darwin GOARCH=amd64 go build -ldflags="$(BUILD_ARGS)" -o build/zarf-mac-intel main.go

build-cli-mac-apple: check-ui ## Build the Zarf CLI for macOS on ARM
	GOOS=darwin GOARCH=arm64 go build -ldflags="$(BUILD_ARGS)" -o build/zarf-mac-apple main.go

build-cli-windows-amd: check-ui ## Build the Zarf CLI for Windows on AMD64
	GOOS=windows GOARCH=amd64 go build -ldflags="$(BUILD_ARGS)" -o build/zarf.exe main.go ## Build the Zarf CLI for Windows on AMD64

build-cli-windows-arm: check-ui ## Build the Zarf CLI for Windows on ARM
	GOOS=windows GOARCH=arm64 go build -ldflags="$(BUILD_ARGS)" -o build/zarf-arm.exe main.go ## Build the Zarf CLI for Windows on ARM

build-cli-linux: build-cli-linux-amd build-cli-linux-arm ## Build the Zarf CLI for Linux on AMD64 and ARM

build-cli: build-cli-linux-amd build-cli-linux-arm build-cli-mac-intel build-cli-mac-apple build-cli-windows-amd build-cli-windows-arm ## Build the CLI

docs-and-schema: ensure-ui-build-dir ## Generate the Zarf Documentation and Schema
	go run main.go internal generate-cli-docs
	.hooks/create-zarf-schema.sh

dev: ensure-ui-build-dir ## Start a Dev Server for the UI
	go mod download
	npm ci
	npm run dev

# Inject and deploy a new dev version of zarf agent for testing (should have an existing zarf agent deployemt)
# @todo: find a clean way to dynamically support Kind or k3d:
#        when using kind: kind load docker-image $(tag)
#        when using k3d: k3d image import $(tag)
dev-agent-image: ## Create a new agent image and inject it into a currently inited cluster
	$(eval tag := defenseunicorns/dev-zarf-agent:$(shell date +%s))
	$(eval arch := $(shell uname -m))
	CGO_ENABLED=0 GOOS=linux go build -o build/zarf-linux-$(arch) main.go
	DOCKER_BUILDKIT=1 docker build --tag $(tag) --build-arg TARGETARCH=$(arch) . && \
	k3d image import $(tag) && \
	kubectl -n zarf set image deployment/agent-hook server=$(tag)

init-package: ## Create the zarf init package (must `brew install coreutils` on macOS first)
	@test -s $(ZARF_BIN) || $(MAKE) build-cli
	$(ZARF_BIN) package create -o build -a $(ARCH) --set AGENT_IMAGE=$(AGENT_IMAGE) --set INJECTOR_TAG=$(ARCH)-$(INJECTOR_VERSION) --confirm .

ci-release: init-package

build-examples: ## Build all of the example packages
	@test -s $(ZARF_BIN) || $(MAKE) build-cli

	@test -s ./build/zarf-package-dos-games-$(ARCH).tar.zst || $(ZARF_BIN) package create examples/game -o build -a $(ARCH) --confirm

	@test -s ./build/zarf-package-component-scripts-$(ARCH).tar.zst || $(ZARF_BIN) package create examples/component-scripts -o build -a $(ARCH) --confirm

	@test -s ./build/zarf-package-component-choice-$(ARCH).tar.zst || $(ZARF_BIN) package create examples/component-choice -o build -a $(ARCH) --confirm

	@test -s ./build/zarf-package-package-variables-$(ARCH).tar.zst || $(ZARF_BIN) package create examples/package-variables --set CONFIG_MAP=simple-configmap.yaml --set ACTION=template -o build -a $(ARCH) --confirm

	@test -s ./build/zarf-package-data-injection-demo-$(ARCH).tar || $(ZARF_BIN) package create examples/data-injection -o build -a $(ARCH) --confirm

	@test -s ./build/zarf-package-git-data-$(ARCH).tar.zst || $(ZARF_BIN) package create examples/git-data -o build -a $(ARCH) --confirm

	@test -s ./build/zarf-package-test-helm-releasename-$(ARCH).tar.zst || $(ZARF_BIN) package create examples/helm-alt-release-name -o build -a $(ARCH) --confirm

	@test -s ./build/zarf-package-test-helm-local-chart-$(ARCH).tar.zst || $(ZARF_BIN) package create examples/helm-local-chart -o build -a $(ARCH) --confirm

	@test -s ./build/zarf-package-compose-example-$(ARCH).tar.zst || $(ZARF_BIN) package create examples/composable-packages -o build -a $(ARCH) --confirm

	@test -s ./build/zarf-package-flux-test-$(ARCH).tar.zst || $(ZARF_BIN) package create examples/flux-test -o build -a $(ARCH) --confirm

	@test -s ./build/zarf-package-test-helm-wait-$(ARCH).tar.zst || $(ZARF_BIN) package create examples/helm-no-wait -o build -a $(ARCH) --confirm

## Run e2e tests. Will automatically build any required dependencies that aren't present.
## Requires an existing cluster for the env var APPLIANCE_MODE=true
.PHONY: test-e2e
test-e2e: build-examples ## Run all of the core Zarf CLI E2E tests
	@test -s ./build/zarf-init-$(ARCH)-$(CLI_VERSION).tar.zst || $(ZARF_BIN) package create -o build -a $(ARCH) --set AGENT_IMAGE=$(AGENT_IMAGE) --confirm .
	@test -s ./build/zarf-init-$(ARCH)-$(CLI_VERSION).tar.zst || $(MAKE) init-package
	cd src/test/e2e && go test -failfast -v -timeout 30m

.PHONY: test-external
test-external: ## Run the Zarf CLI E2E tests for an external registry and cluster
	@test -s $(ZARF_BIN) || $(MAKE) build-cli
	@test -s ./build/zarf-init-$(ARCH)-$(CLI_VERSION).tar.zst || $(MAKE) init-package
	@test -s ./build/zarf-package-flux-test-$(ARCH).tar.zst || $(ZARF_BIN) package create examples/flux-test -o build -a $(ARCH) --confirm
	cd src/test/external-test && go test -failfast -v -timeout 30m

test-built-ui: ## Run the Zarf UI E2E tests (requires `make build-ui` first)
	API_PORT=3333 API_TOKEN=insecure $(ZARF_BIN) dev ui

test-docs-and-schema:
	$(MAKE) docs-and-schema
	.hooks/check-zarf-docs-and-schema.sh

test-cves: ensure-ui-build-dir
	go run main.go tools sbom packages . -o json | grype --fail-on low

cve-report: ensure-ui-build-dir
	go run main.go tools sbom packages . -o json | grype -o template -t .hooks/grype.tmpl > build/zarf-known-cves.csv
