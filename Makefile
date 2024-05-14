# SPDX-License-Identifier: Apache-2.0
# SPDX-FileCopyrightText: 2021-Present The Zarf Authors

# Provide a default value for the operating system architecture used in tests, e.g. " APPLIANCE_MODE=true|false make test-e2e ARCH=arm64"
ARCH ?= amd64
######################################################################################

# Figure out which Zarf binary we should use based on the operating system we are on
ZARF_BIN := ./build/zarf
BUILD_CLI_FOR_SYSTEM := build-cli-linux-amd
ifeq ($(OS),Windows_NT)
	ZARF_BIN := $(addsuffix .exe,$(ZARF_BIN))
	BUILD_CLI_FOR_SYSTEM := build-cli-windows-amd
else
	UNAME_S := $(shell uname -s)
	UNAME_P := $(shell uname -p)
	ifneq ($(UNAME_S),Linux)
		ifeq ($(UNAME_S),Darwin)
			ZARF_BIN := $(addsuffix -mac,$(ZARF_BIN))
		endif
		ifeq ($(UNAME_P),i386)
			ZARF_BIN := $(addsuffix -intel,$(ZARF_BIN))
			BUILD_CLI_FOR_SYSTEM = build-cli-mac-intel
		endif
		ifeq ($(UNAME_P),arm)
			ZARF_BIN := $(addsuffix -apple,$(ZARF_BIN))
			BUILD_CLI_FOR_SYSTEM = build-cli-mac-apple
		endif
	endif
endif

CLI_VERSION ?= $(if $(shell git describe --tags),$(shell git describe --tags),"UnknownVersion")
BUILD_ARGS := -s -w -X github.com/defenseunicorns/zarf/src/config.CLIVersion=$(CLI_VERSION)
K8S_MODULES_VER=$(subst ., ,$(subst v,,$(shell go list -f '{{.Version}}' -m k8s.io/client-go)))
K8S_MODULES_MAJOR_VER=$(shell echo $$(($(firstword $(K8S_MODULES_VER)) + 1)))
K8S_MODULES_MINOR_VER=$(word 2,$(K8S_MODULES_VER))
K8S_MODULES_PATCH_VER=$(word 3,$(K8S_MODULES_VER))
K9S_VERSION=$(shell go list -f '{{.Version}}' -m github.com/derailed/k9s)
CRANE_VERSION=$(shell go list -f '{{.Version}}' -m github.com/google/go-containerregistry)
SYFT_VERSION=$(shell go list -f '{{.Version}}' -m github.com/anchore/syft)
ARCHIVER_VERSION=$(shell go list -f '{{.Version}}' -m github.com/mholt/archiver/v3)
HELM_VERSION=$(shell go list -f '{{.Version}}' -m helm.sh/helm/v3)

BUILD_ARGS += -X helm.sh/helm/v3/pkg/lint/rules.k8sVersionMajor=$(K8S_MODULES_MAJOR_VER)
BUILD_ARGS += -X helm.sh/helm/v3/pkg/lint/rules.k8sVersionMinor=$(K8S_MODULES_MINOR_VER)
BUILD_ARGS += -X helm.sh/helm/v3/pkg/chartutil.k8sVersionMajor=$(K8S_MODULES_MAJOR_VER)
BUILD_ARGS += -X helm.sh/helm/v3/pkg/chartutil.k8sVersionMinor=$(K8S_MODULES_MINOR_VER)
BUILD_ARGS += -X k8s.io/component-base/version.gitVersion=v$(K8S_MODULES_MAJOR_VER).$(K8S_MODULES_MINOR_VER).$(K8S_MODULES_PATCH_VER)
BUILD_ARGS += -X github.com/derailed/k9s/cmd.version=$(K9S_VERSION)
BUILD_ARGS += -X github.com/google/go-containerregistry/cmd/crane/cmd.Version=$(CRANE_VERSION)
BUILD_ARGS += -X github.com/defenseunicorns/zarf/src/cmd/tools.syftVersion=$(SYFT_VERSION)
BUILD_ARGS += -X github.com/defenseunicorns/zarf/src/cmd/tools.archiverVersion=$(ARCHIVER_VERSION)
BUILD_ARGS += -X github.com/defenseunicorns/zarf/src/cmd/tools.helmVersion=$(HELM_VERSION)

GIT_SHA := $(if $(shell git rev-parse HEAD),$(shell git rev-parse HEAD),"")
BUILD_DATE := $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
BUILD_ARGS += -X k8s.io/component-base/version.gitCommit=$(GIT_SHA)
BUILD_ARGS += -X k8s.io/component-base/version.buildDate=$(BUILD_DATE)

.DEFAULT_GOAL := build

.PHONY: help
help: ## Display this help information
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| sort | awk 'BEGIN {FS = ":.*?## "}; \
		{printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

clean: ## Clean the build directory
	rm -rf build

destroy: ## Run `zarf destroy` on the current cluster
	$(ZARF_BIN) destroy --confirm --remove-components
	rm -fr build

delete-packages: ## Delete all Zarf package tarballs in the project recursively
	find . -type f -name 'zarf-package-*' -delete

# Note: the path to the main.go file is not used due to https://github.com/golang/go/issues/51831#issuecomment-1074188363
.PHONY: build
build: ## Build the Zarf CLI for the machines OS and architecture
	go mod tidy
	$(MAKE) $(BUILD_CLI_FOR_SYSTEM)

build-cli-linux-amd: ## Build the Zarf CLI for Linux on AMD64
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="$(BUILD_ARGS)" -o build/zarf .

build-cli-linux-arm: ## Build the Zarf CLI for Linux on ARM
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="$(BUILD_ARGS)" -o build/zarf-arm .

build-cli-mac-intel: ## Build the Zarf CLI for macOS on AMD64
	GOOS=darwin GOARCH=amd64 go build -ldflags="$(BUILD_ARGS)" -o build/zarf-mac-intel .

build-cli-mac-apple: ## Build the Zarf CLI for macOS on ARM
	GOOS=darwin GOARCH=arm64 go build -ldflags="$(BUILD_ARGS)" -o build/zarf-mac-apple .

build-cli-windows-amd: ## Build the Zarf CLI for Windows on AMD64
	GOOS=windows GOARCH=amd64 go build -ldflags="$(BUILD_ARGS)" -o build/zarf.exe . ## Build the Zarf CLI for Windows on AMD64

build-cli-windows-arm: ## Build the Zarf CLI for Windows on ARM
	GOOS=windows GOARCH=arm64 go build -ldflags="$(BUILD_ARGS)" -o build/zarf-arm.exe . ## Build the Zarf CLI for Windows on ARM

build-cli-linux: build-cli-linux-amd build-cli-linux-arm ## Build the Zarf CLI for Linux on AMD64 and ARM

build-cli: build-cli-linux-amd build-cli-linux-arm build-cli-mac-intel build-cli-mac-apple build-cli-windows-amd build-cli-windows-arm ## Build the CLI

docs-and-schema: ## Generate the Zarf Documentation and Schema
	ZARF_CONFIG=hack/empty-config.toml go run main.go internal gen-cli-docs
	ZARF_CONFIG=hack/empty-config.toml hack/create-zarf-schema.sh

lint-packages-and-examples: build ## Recursively lint all zarf.yaml files in the repo except for those dedicated to tests
	hack/lint-all-zarf-packages.sh $(ZARF_BIN) false

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
		--set REGISTRY_IMAGE_TAG="2.8.3"

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

	@test -s ./build/zarf-package-component-webhooks-$(ARCH)-0.0.1.tar.zst || $(ZARF_BIN) package create examples/component-webhooks -o build -a $(ARCH) --confirm

build-injector-linux: ## Build the Zarf injector for AMD64 and ARM64
	docker run --rm --user "$(id -u)":"$(id -g)" -v $$PWD/src/injector:/usr/src/zarf-injector -w /usr/src/zarf-injector rust:1.71.0-bookworm make build-injector-linux list-sizes

## NOTE: Requires an existing cluster or the env var APPLIANCE_MODE=true
.PHONY: test-e2e
test-e2e: test-e2e-without-cluster test-e2e-with-cluster  ## Run all of the core Zarf CLI E2E tests (builds any deps that aren't present)

.PHONY: test-e2e-with-cluster
test-e2e-with-cluster: build-examples ## Run all of the core Zarf CLI E2E tests that DO require a cluster (builds any deps that aren't present)
	@test -s ./build/zarf-init-$(ARCH)-$(CLI_VERSION).tar.zst || $(MAKE) init-package
	cd src/test/e2e && go test ./main_test.go ./[2-9]*.go -failfast -v -timeout 35m

.PHONY: test-e2e-without-cluster
test-e2e-without-cluster: build-examples ## Run all of the core Zarf CLI E2E tests  that DO NOT require a cluster (builds any deps that aren't present)
	@test -s ./build/zarf-init-$(ARCH)-$(CLI_VERSION).tar.zst || $(MAKE) init-package
	cd src/test/e2e && go test ./main_test.go ./[01]* -failfast -v -timeout 35m

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
test-unit: ## Run unit tests
	cd src/pkg && go test ./... -failfast -v -timeout 30m
	cd src/internal && go test ./... -failfast -v timeout 30m
	cd src/extensions/bigbang && go test ./. -failfast -v timeout 30m

# INTERNAL: used to test that a dev has ran `make docs-and-schema` in their PR
test-docs-and-schema:
	$(MAKE) docs-and-schema
	hack/check-zarf-docs-and-schema.sh

# INTERNAL: used to test for new CVEs that may have been introduced
test-cves:
	go run main.go tools sbom scan . -o json --exclude './site' --exclude './examples' | grype --fail-on low

cve-report: ## Create a CVE report for the current project (must `brew install grype` first)
	@test -d ./build || mkdir ./build
	go run main.go tools sbom scan . -o json --exclude './site' --exclude './examples' | grype -o template -t hack/grype.tmpl > build/zarf-known-cves.csv

lint-go: ## Run revive to lint the go code (must `brew install revive` first)
	revive -config hack/revive.toml -exclude src/cmd/viper.go -formatter stylish ./src/...
	@hack/check-spdx-go.sh src >/dev/null || (echo "SPDX check for go failed, please run 'hack/check-spdx-go.sh src' to see the errors" && exit 1)
