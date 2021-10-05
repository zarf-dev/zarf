# Figure out which Zarf binary we should use based on the operating system we are on
ZARF_BIN := ./build/zarf
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

remove-packages: ## remove all zarf packages recursively
	find . -type f -name 'zarf-package-*' -delete

vm-init: ## usage -> make vm-init OS=ubuntu
	vagrant destroy -f
	vagrant up --no-color ${OS}
	echo -e "\n\n\n\033[1;93m  ✅ BUILD COMPLETE.  To access this environment, run \"vagrant ssh ${OS}\"\n\n\n"

vm-destroy: ## Destroy the VM
	vagrant destroy -f

test-e2e: ## Run E2E tests. Requires access to an AWS account. Costs money.
	cd test/e2e && go test ./... -v -timeout 1200s

test-ssh: ## Run this if you have set SKIP_teardown=1 and want to SSH into the still-running test server. Don't forget to unset SKIP_teardown when you're done
	cd test/tf/public-ec2-instance/.test-data && cat Ec2KeyPair.json | jq -r .PrivateKey > privatekey.pem && chmod 600 privatekey.pem
	cd test/tf/public-ec2-instance && ssh -i .test-data/privatekey.pem ubuntu@$$(terraform output public_instance_ip)

build-cli: ## Build the CLI
	rm -fr build
	cd cli && $(MAKE) build
	cd cli && $(MAKE) build-mac

init-package: ## Create the zarf init package
	$(ZARF_BIN) package create --confirm
	mv zarf-init.tar.zst build

	cd build && sha256sum -b zarf* > zarf.sha256
	ls -lh build

build-test: build-cli init-package ## Build the CLI and create the init package

ci-release: init-package ## Create the init package

package-examples: ## automatically package all example directories and add the tarballs to the examples/sync directory
	cd examples && $(MAKE) package-examples
