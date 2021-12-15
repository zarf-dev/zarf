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

.PHONY: vendor-big-bang-base
vendor-big-bang-base: ## Grab the bigbang base kustomization so we don't need to do funky things to let Flux grab it from a private repo
	cd template/bigbang/vendor && \
	rm -rf bigbang && \
	git init bigbang && \
	cd bigbang && \
	git remote add -f origin https://repo1.dso.mil/platform-one/big-bang/bigbang.git && \
	git config core.sparseCheckout true && \
	echo "base/" > .git/info/sparse-checkout && \
	git checkout tags/1.17.0 -b tagbranch && \
	rm -rf .git

.PHONY: package-example-istio-with-preloaded-cert
package-example-istio-with-preloaded-cert: vendor-big-bang-base ## Example that deploys Istio with a preloaded certificate
	kustomize build template/bigbang/vendor/bigbang/base/flux > manifests/flux/flux-generated.yaml && $(ZARF_BIN) package create --confirm && mv zarf-package-* ../sync/

.PHONY: all
all: ## Download the latest version of Zarf, build the deploy package, and start a VM with Vagrant
	@cd .. && $(MAKE) clean fetch-release
	@$(MAKE) package-example-istio-with-preloaded-cert vm-init

.PHONY: all-dev
all-dev: ## Same as 'default', but build Zarf rather than downloading it
	@cd .. && $(MAKE) clean build-release
	@$(MAKE) package-example-istio-with-preloaded-cert vm-init

.PHONY: vm-init
vm-init: ## Bring up the VM
	@cd .. && $(MAKE) vm-init

.PHONY: vm-destroy
vm-destroy: ## Destroy the VM
	@cd .. && $(MAKE) vm-destroy
