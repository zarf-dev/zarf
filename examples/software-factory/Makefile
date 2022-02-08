# Figure out which Zarf binary we should use based on the operating system we are on
ZARF_BIN := ../sync/zarf
UNAME_S := $(shell uname -s)
UNAME_P := $(shell uname -p)
ifneq ($(UNAME_S),Linux)
	ifeq ($(UNAME_S),Darwin)
		ZARF_BIN := $(addsuffix -mac,$(ZARF_BIN))
	endif
	ifeq ($(UNAME_P),i386)
		ZARF_BIN := $(addsuffix -intel,$(ZARF_BIN))
	endif
	ifeq ($(UNAME_P),arm64)
		ZARF_BIN := $(addsuffix -apple,$(ZARF_BIN))
	endif
endif

.DEFAULT_GOAL := help


.PHONY: help
help: ## Show a list of all targets
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
	| sed -n 's/^\(.*\): \(.*\)##\(.*\)/\1:\3/p' \
	| column -t -s ":"

.PHONY: all
all: clean fetch-release package-example-software-factory vm-init ## Download zarf, build all packages and launch a basic VM with the assets

.PHONY: all-dev
all-dev: clean build-release package-example-software-factory vm-init ## Same as target 'all', but build the binaries using the current codebase rather than downloading the latest version from the internet

.PHONY: clean
clean: ## Clean the sync dir
	@cd .. && $(MAKE) clean

.PHONY: fetch-release
fetch-release: ## Grab the latest release as an alternative to needing to build the binaries
	@cd .. && $(MAKE) fetch-release

.PHONY: build-release
build-release: ## Build the binaries as an alternative to downloading the latest release
	@cd .. && $(MAKE) build-release

.PHONY: vm-init
vm-init: vm-destroy ## Stripped-down vagrant box to reduce friction for basic user testing. Note the need to perform disk resizing for some examples
	@cd .. && $(MAKE) vm-init

.PHONY: vm-destroy
vm-destroy: ## Cleanup plz
	@cd .. && $(MAKE) vm-destroy

.PHONY: package-example-software-factory
package-example-software-factory: ## Create the software factory deploy package
	@kustomize build template/bigbang > manifests/bigbang/bigbang-generated.yaml && kustomize build template/flux > manifests/flux/flux-generated.yaml && $(ZARF_BIN) package create --confirm && mv zarf-package-* ../sync/

.PHONY: ssh
ssh: ## SSH into the Vagrant VM
	@cd .. && vagrant ssh
