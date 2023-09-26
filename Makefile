# SPDX-License-Identifier: Apache-2.0
# SPDX-FileCopyrightText: 2021-Present The Zarf Authors

# Provide a default value for the operating system architecture used in tests, e.g. " APPLIANCE_MODE=true|false make test-e2e ARCH=arm64"
ARCH ?= amd64
KEY ?= ""
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

CLI_VERSION ?= $(if $(shell git describe --tags),$(shell git describe --tags),"UnknownVersion")
GIT_SHA := $(if $(shell git rev-parse HEAD),$(shell git rev-parse HEAD),"")
BUILD_DATE := $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
BUILD_ARGS := -s -w -X 'github.com/defenseunicorns/zarf/src/config.CLIVersion=$(CLI_VERSION)' -X 'k8s.io/component-base/version.gitVersion=v0.0.0+zarf$(CLI_VERSION)' -X 'k8s.io/component-base/version.gitCommit=$(GIT_SHA)' -X 'k8s.io/component-base/version.buildDate=$(BUILD_DATE)'
.DEFAULT_GOAL := help

.PHONY: help
help: ## Display this help information
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
	  | sort | awk 'BEGIN {FS = ":.*?## "}; \
	  {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

clean: ## Clean the build directory
	rm -rf build

destroy: ## Run `zarf destroy` on the current cluster
	$(ZARF_BIN) destroy --confirm --remove-components
	rm -fr build

delete-packages: ## Delete all Zarf package tarballs in the project recursively
	find . -type f -name 'zarf-package-*' -delete

# INTERNAL: used to ensure the ui directory exists
ensure-ui-build-dir:
	mkdir -p build/ui
	touch build/ui/index.html

# INTERNAL: used to build the UI only if necessary
check-ui:
	@ if [ ! -z "$(shell command -v shasum)" ]; then\
		if test "$(shell ./hack/print-ui-diff.sh | shasum)" != "$(shell cat build/ui/git-info.txt | shasum)" ; then\
			$(MAKE) build-ui;\
			./hack/print-ui-diff.sh > build/ui/git-info.txt;\
		fi;\
	else\
		$(MAKE) build-ui;\
	fi

build-ui: ## Build the Zarf UI
	npm --prefix src/ui ci
	npm --prefix src/ui run build

# Note: the path to the main.go file is not used due to https://github.com/golang/go/issues/51831#issuecomment-1074188363

build-cli-linux-amd: check-ui ## Build the Zarf CLI for Linux on AMD64
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="$(BUILD_ARGS)" -o build/zarf .

build-cli-linux-arm: check-ui ## Build the Zarf CLI for Linux on ARM
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="$(BUILD_ARGS)" -o build/zarf-arm .

build-cli-mac-intel: check-ui ## Build the Zarf CLI for macOS on AMD64
	GOOS=darwin GOARCH=amd64 go build -ldflags="$(BUILD_ARGS)" -o build/zarf-mac-intel .

build-cli-mac-apple: check-ui ## Build the Zarf CLI for macOS on ARM
	GOOS=darwin GOARCH=arm64 go build -ldflags="$(BUILD_ARGS)" -o build/zarf-mac-apple .

build-cli-windows-amd: check-ui ## Build the Zarf CLI for Windows on AMD64
	GOOS=windows GOARCH=amd64 go build -ldflags="$(BUILD_ARGS)" -o build/zarf.exe . ## Build the Zarf CLI for Windows on AMD64

build-cli-windows-arm: check-ui ## Build the Zarf CLI for Windows on ARM
	GOOS=windows GOARCH=arm64 go build -ldflags="$(BUILD_ARGS)" -o build/zarf-arm.exe . ## Build the Zarf CLI for Windows on ARM

build-cli-linux: build-cli-linux-amd build-cli-linux-arm ## Build the Zarf CLI for Linux on AMD64 and ARM

build-cli: build-cli-linux-amd build-cli-linux-arm build-cli-mac-intel build-cli-mac-apple build-cli-windows-amd build-cli-windows-arm ## Build the CLI

docs-and-schema: ensure-ui-build-dir ## Generate the Zarf Documentation and Schema
	hack/gen-cli-docs.sh
	ZARF_CONFIG=hack/empty-config.toml hack/create-zarf-schema.sh

dev: ensure-ui-build-dir ## Start a Dev Server for the Zarf UI
	go mod download
	npm --prefix src/ui ci
	npm --prefix src/ui run dev

# INTERNAL: a shim used to build the agent image only if needed on Windows using the `test` command
init-package-local-agent:
	@test "$(AGENT_IMAGE_TAG)" != "local" || $(MAKE) build-local-agent-image

build-local-agent-image: ## Build the Zarf agent image to be used in a locally built init package
	@ if [ "$(ARCH)" = "amd64" ] && [ ! -s ./build/zarf ]; then $(MAKE) build-cli-linux-amd; fi
	@ if [ "$(ARCH)" = "amd64" ]; then cp build/zarf build/zarf-linux-amd64; fi
	@ if [ "$(ARCH)" = "arm64" ] && [ ! -s ./build/zarf-arm ]; then $(MAKE) build-cli-linux-arm; fi
	@ if [ "$(ARCH)" = "arm64" ]; then cp build/zarf-arm build/zarf-linux-arm64; fi
	docker buildx build --load --platform linux/$(ARCH) --tag ghcr.io/defenseunicorns/zarf/agent:local .
	@ if [ "$(ARCH)" = "amd64" ]; then rm build/zarf-linux-amd64; fi
	@ if [ "$(ARCH)" = "arm64" ]; then rm build/zarf-linux-arm64; fi

init-package: ## Create the zarf init package (must `brew install coreutils` on macOS and have `docker` first)
	@test -s $(ZARF_BIN) || $(MAKE) build-cli
	$(ZARF_BIN) package create -o build -a $(ARCH) --confirm .

# INTERNAL: used to build a release version of the init package with a specific agent image
release-init-package:
	$(ZARF_BIN) package create -o build -a $(ARCH) --set AGENT_IMAGE_TAG=$(AGENT_IMAGE_TAG) --confirm .

# INTERNAL: used to build an iron bank version of the init package with an ib version of the registry image
ib-init-package:
	@test -s $(ZARF_BIN) || $(MAKE) build-cli
	$(ZARF_BIN) package create -o build -a $(ARCH) --confirm . \
		--set REGISTRY_IMAGE_DOMAIN="registry1.dso.mil/" \
		--set REGISTRY_IMAGE="ironbank/opensource/docker/registry-v2" \
		--set REGISTRY_IMAGE_TAG="2.8.2"

# INTERNAL: used to publish the init package
publish-init-package:
	$(ZARF_BIN) package publish build/zarf-init-$(ARCH)-$(CLI_VERSION).tar.zst oci://$(REPOSITORY_URL)
	$(ZARF_BIN) package publish . oci://$(REPOSITORY_URL)

build-examples: ## Build all of the example packages
	@test -s $(ZARF_BIN) || $(MAKE) build-cli

	@test -s ./build/zarf-package-dos-games-$(ARCH)-1.0.0.tar.zst || $(ZARF_BIN) package create examples/dos-games -o build -a $(ARCH) --confirm

	@test -s ./build/zarf-package-manifests-$(ARCH)-0.0.1.tar.zst || $(ZARF_BIN) package create examples/manifests -o build -a $(ARCH) --confirm

	@test -s ./build/zarf-package-component-actions-$(ARCH).tar.zst || $(ZARF_BIN) package create examples/component-actions -o build -a $(ARCH) --confirm

	@test -s ./build/zarf-package-component-choice-$(ARCH).tar.zst || $(ZARF_BIN) package create examples/component-choice -o build -a $(ARCH) --confirm

	@test -s ./build/zarf-package-variables-$(ARCH).tar.zst || $(ZARF_BIN) package create examples/variables --set NGINX_VERSION=1.23.3 -o build -a $(ARCH) --confirm

	@test -s ./build/zarf-package-kiwix-$(ARCH)-3.5.0.tar || $(ZARF_BIN) package create examples/kiwix -o build -a $(ARCH) --confirm

	@test -s ./build/zarf-package-git-data-$(ARCH)-0.0.1.tar.zst || $(ZARF_BIN) package create examples/git-data -o build -a $(ARCH) --confirm

	@test -s ./build/zarf-package-helm-charts-$(ARCH)-0.0.1.tar.zst || $(ZARF_BIN) package create examples/helm-charts -o build -a $(ARCH) --confirm

	@test -s ./build/zarf-package-podinfo-flux-$(ARCH).tar.zst || $(ZARF_BIN) package create examples/podinfo-flux -o build -a $(ARCH) --confirm

	@test -s ./build/zarf-package-argocd-$(ARCH).tar.zst || $(ZARF_BIN) package create examples/argocd -o build -a $(ARCH) --confirm

	@test -s ./build/zarf-package-yolo-$(ARCH).tar.zst || $(ZARF_BIN) package create examples/yolo -o build -a $(ARCH) --confirm

build-injector-linux: ## Build the Zarf injector for AMD64 and ARM64
	docker run --rm --user "$(id -u)":"$(id -g)" -v $$PWD/src/injector:/usr/src/zarf-injector -w /usr/src/zarf-injector rust:1.71.0-bookworm make build-injector-linux

## NOTE: Requires an existing cluster or the env var APPLIANCE_MODE=true
.PHONY: test-e2e
test-e2e: build-examples ## Run all of the core Zarf CLI E2E tests (builds any deps that aren't present)
	@test -s ./build/zarf-init-$(ARCH)-$(CLI_VERSION).tar.zst || $(MAKE) init-package
	cd src/test/e2e && go test -failfast -v -timeout 35m

## NOTE: Requires an existing cluster
.PHONY: test-external
test-external: ## Run the Zarf CLI E2E tests for an external registry and cluster
	@test -s $(ZARF_BIN) || $(MAKE) build-cli
	@test -s ./build/zarf-init-$(ARCH)-$(CLI_VERSION).tar.zst || $(MAKE) init-package
	@test -s ./build/zarf-package-podinfo-flux-$(ARCH).tar.zst || $(ZARF_BIN) package create examples/podinfo-flux -o build -a $(ARCH) --confirm
	@test -s ./build/zarf-package-argocd-$(ARCH).tar.zst || $(ZARF_BIN) package create examples/argocd -o build -a $(ARCH) --confirm
	cd src/test/external && go test -failfast -v -timeout 30m

## NOTE: Requires an existing cluster and
.PHONY: test-upgrade
test-upgrade: ## Run the Zarf CLI E2E tests for an external registry and cluster
	@test -s $(ZARF_BIN) || $(MAKE) build-cli
	[ -n "$(shell zarf version)" ] || (echo "Zarf must be installed prior to the upgrade test" && exit 1)
	[ -n "$(shell zarf package list 2>&1 | grep test-upgrade-package)" ] || (echo "Zarf must be initialized and have the 6.3.3 upgrade-test package installed prior to the upgrade test" && exit 1)
	@test -s "zarf-package-test-upgrade-package-amd64-6.3.4.tar.zst" || zarf package create src/test/upgrade/ --set PODINFO_VERSION=6.3.4 --confirm
	cd src/test/upgrade && go test -failfast -v -timeout 30m

.PHONY: test-unit
test-unit: ensure-ui-build-dir ## Run unit tests within the src/pkg and the bigbang extension directory
	cd src/pkg && go test ./... -failfast -v -timeout 30m
	cd src/extensions/bigbang && go test ./. -failfast -v timeout 30m

.PHONY: test-ui
test-ui: ## Run the Zarf UI E2E tests (requires `make build-ui` first) (run with env CI=true to use build/zarf)
	export NODE_PATH=$(CURDIR)/src/ui/node_modules && \
	npm --prefix src/ui run test:pre-init && \
	npm --prefix src/ui run test:init && \
	npm --prefix src/ui run test:post-init && \
	npm --prefix src/ui run test:connect

.PHONY: test-ui-dev-server
# INTERNAL: used to start a dev version of the API server for the Zarf Web UI tests (locally)
test-ui-dev-server:
	API_DEV_PORT=5173 \
		API_PORT=3333 \
		API_TOKEN=insecure \
		go run -ldflags="$(BUILD_ARGS)" main.go dev ui -l=trace

.PHONY: test-ui-build-server
# INTERNAL: used to start the built version of the API server for the Zarf Web UI (in CI)
test-ui-build-server:
	API_PORT=3333 API_TOKEN=insecure $(ZARF_BIN) dev ui

# INTERNAL: used to test that a dev has ran `make docs-and-schema` in their PR
test-docs-and-schema:
	$(MAKE) docs-and-schema
	hack/check-zarf-docs-and-schema.sh

# INTERNAL: used to test for new CVEs that may have been introduced
test-cves: ensure-ui-build-dir
	go run main.go tools sbom packages . -o json --exclude './docs-website' | grype --fail-on low

cve-report: ensure-ui-build-dir ## Create a CVE report for the current project (must `brew install grype` first)
	go run main.go tools sbom packages . -o json --exclude './docs-website' | grype -o template -t hack/.templates/grype.tmpl > build/zarf-known-cves.csv

lint-go: ## Run revive to lint the go code (must `brew install revive` first)
	revive -config revive.toml -exclude src/cmd/viper.go -formatter stylish ./src/...
