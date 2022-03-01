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
all: clean fetch-release package-examples vm-init ## Download zarf, build all packages and launch a basic VM with the assets

.PHONY: all-dev
all-dev: clean build-release package-examples vm-init ## Same as target 'all', but build the binaries using the current codebase rather than downloading the latest version from the internet

.PHONY: clean
clean: ## Clean the sync dir
	@rm -fr sync && mkdir -p sync

.PHONY: fetch-release
fetch-release: ## Grab the latest release as an alternative to needing to build the binaries
	@mkdir -p sync
	@# This probably isn't the cleanest way to get a release, but since we're moving to github, not worth adding the code until post-migration
	@curl -fL "https://zarf-public.s3-us-gov-west-1.amazonaws.com/release/$$(git describe --tags --abbrev=0)/{zarf,zarf-mac-intel,zarf-mac-apple,zarf-init.tar.zst}" -o "sync/#1"
	@chmod +x sync/*

.PHONY: build-release
build-release: ## Build the binaries as an alternative to downloading the latest release
	@cd .. && $(MAKE) build-test
	@mkdir -p sync
	@cp -R ../build/* sync

.PHONY: vm-init
vm-init: vm-destroy ## Stripped-down vagrant box to reduce friction for basic user testing. Note the need to perform disk resizing for some examples
	@VAGRANT_EXPERIMENTAL="disks" vagrant up --no-color
	@vagrant ssh

.PHONY: vm-destroy
vm-destroy: ## Cleanup plz
	@vagrant destroy -f

.PHONY: package-examples
package-examples: package-example-big-bang package-example-software-factory package-example-data-injection package-example-game package-example-gitops-data package-example-single-big-bang-package package-example-tiny-kafka package-example-postgres-operator package-example-composability ## Create zarf packages from all examples

.PHONY: package-example-big-bang
package-example-big-bang: ## Create the Big Bang Core example
	cd big-bang && $(ZARF_BIN) package create --confirm && mv zarf-package-* ../sync/

.PHONY: generate-bigbang-dev-cert
generate-bigbang-dev-cert: ## Download the TLS cert and key for the *.bigbang.dev domain
	@mkdir -p istio-with-separate-cert/files
	@curl -s https://repo1.dso.mil/platform-one/big-bang/bigbang/-/raw/master/chart/ingress-certs.yaml | yq .istio.gateways.public.tls.key > istio-with-separate-cert/files/bigbangdev.key
	@curl -s https://repo1.dso.mil/platform-one/big-bang/bigbang/-/raw/master/chart/ingress-certs.yaml | yq .istio.gateways.public.tls.cert > istio-with-separate-cert/files/bigbangdev.cert

.PHONY: package-example-istio-with-separate-cert
package-example-istio-with-separate-cert: ## Create the example of Istio with a separate TLS cert
	@cd istio-with-separate-cert && $(ZARF_BIN) package create --confirm && mv zarf-package-* ../sync/
	@cp istio-with-separate-cert/files/bigbangdev.cert sync/bigbangdev.cert
	@cp istio-with-separate-cert/files/bigbangdev.key sync/bigbangdev.key

.PHONY: package-example-softare-factory
package-example-software-factory: ## Create the Big Bang Software Factory example
	cd software-factory && kustomize build template/bigbang > manifests/bigbang/bigbang-generated.yaml && kustomize build template/flux > manifests/flux/flux-generated.yaml && $(ZARF_BIN) package create --confirm && mv zarf-package-* ../sync/

.PHONY: package-example-data-injection
package-example-data-injection: ## Create the Data Injection example
	cd data-injection && $(ZARF_BIN) package create --confirm && mv zarf-package-* ../sync/

.PHONY: package-example-game
package-example-game: ## Create the Doom example
	cd game && $(ZARF_BIN) package create --confirm && mv zarf-package-* ../sync/

.PHONY: package-example-gitops-data
package-example-gitops-data: ## Create the gitops-data example
	cd gitops-data && $(ZARF_BIN) package create --confirm && mv zarf-package-* ../sync/

.PHONY: package-example-single-big-bang-package
package-example-single-big-bang-package: ## Create the Single Big Bang Package example
	cd single-big-bang-package && $(ZARF_BIN) package create --confirm && mv zarf-package-* ../sync/

.PHONY: package-example-tiny-kafka
package-example-tiny-kafka: ## Create the Tiny Kafka example
	cd tiny-kafka && $(ZARF_BIN) package create --confirm && mv zarf-package-* ../sync/

.PHONY: package-example-postgres-operator
package-example-postgres-operator: ## Create the Postgres Operator example
	cd postgres-operator && $(ZARF_BIN) package create --confirm && mv zarf-package-* ../sync/

.PHONY: package-example-composability
package-example-composability: ## Create the Composable example
	cd composable-packages && $(ZARF_BIN) package create --confirm && mv zarf-package-* ../sync/
