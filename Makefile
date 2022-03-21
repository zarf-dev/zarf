# Figure out which Zarf binary we should use based on the operating system we are on
ZARF_BIN := ./build/zarf
UNAME_S := $(shell uname -s)
UNAME_P := $(shell uname -p)
# Need a clean way to map this, arch and uname -a return x86_64 for amd64
ARCH := amd64
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

e2e-ssh: ## Run this if you set SKIP_teardown=1 and want to SSH into the still-running test server. Don't forget to unset SKIP_teardown when you're done
	cd test/tf/public-ec2-instance/.test-data && cat Ec2KeyPair.json | jq -r .PrivateKey > privatekey.pem && chmod 600 privatekey.pem
	cd test/tf/public-ec2-instance && ssh -i .test-data/privatekey.pem ubuntu@$$(terraform output public_instance_ip | tr -d '"')

clean: ## Clean the build dir
	rm -rf build

build-cli-linux: ## Build the Linux CLI
	cd cli && $(MAKE) build

build-cli-mac: ## Build the Mac CLI
	cd cli && $(MAKE) build-mac

build-cli: build-cli-linux build-cli-mac ## Build the CLI

init-package: ## Create the zarf init package, macos "brew install coreutils" first
	$(ZARF_BIN) package create --confirm --architecture amd64
	$(ZARF_BIN) package create --confirm --architecture arm64
	mv zarf-init-*.tar.zst build
	cd build && sha256sum -b zarf* > zarf.sha256
	ls -lh build

build-test: build-cli init-package ## Build the CLI and create the init package

ci-release: init-package ## Create the init package

.PHONY: package-example-game
package-example-game: ## Create the Doom example
	cd examples/game && ../../$(ZARF_BIN) package create --confirm && mv zarf-package-* ../../build/

.PHONY: package-example-data-injection
package-example-data-injection: ## create the Zarf package for the data injection example
	cd examples/data-injection && ../../$(ZARF_BIN) package create --confirm && mv zarf-package-* ../../build/

.PHONY: package-example-single-big-bang-package
package-example-single-big-bang-package: ## Create the Zarf package for single-big-bang-package example
	cd examples/single-big-bang-package && ../../$(ZARF_BIN) package create --confirm && mv zarf-package-* ../../build/

.PHONY: package-example-gitops-data
package-example-gitops-data:
	cd examples/gitops-data && ../../$(ZARF_BIN) package create --confirm && mv zarf-package-* ../../build/

.PHONY: package-example-tiny-kafka
package-example-tiny-kafka:
	cd examples/tiny-kafka && ../../$(ZARF_BIN) package create --confirm && mv zarf-package-* ../../build/

.PHONY: package-example-compose
package-example-compose:
	cd examples/composable-packages && ../../$(ZARF_BIN) package create --confirm && mv zarf-package-* ../../build/

# TODO: This can be cleaned up a little more when `zarf init` is able to provide the path to the `zarf-init-<arch>.tar.zst`
.PHONY: test-e2e
test-e2e: ## Run e2e tests. Will automatically build any required dependencies that aren't present. Requires env var TESTDISTRO=[provided|kind|k3d|k3s]
	@#Check to make sure all the packages we need exist
	@if [ ! -f $(ZARF_BIN) ]; then\
		$(MAKE) build-cli;\
	fi
	@if [ ! -f ./build/zarf-init-$(ARCH).tar.zst ]; then\
		$(MAKE) init-package;\
	fi
	@if [ ! -f ./build/zarf-package-appliance-demo-multi-games-$(ARCH).tar.zst ]; then\
		$(MAKE) package-example-game;\
	fi
	@if [ ! -f ./build/zarf-package-data-injection-demo-$(ARCH).tar ]; then\
		$(MAKE) package-example-data-injection;\
	fi
	@if [ ! -f ./build/zarf-package-gitops-service-data-$(ARCH).tar.zst ]; then\
		$(MAKE) package-example-gitops-data;\
	fi
	@if [ ! -f ./build/zarf-package-compose-example-$(ARCH).tar.zst ]; then\
		$(MAKE) package-example-compose;\
	fi
	cd test/e2e && cp ../../build/zarf-init-$(ARCH).tar.zst . && go test ./... -v -count=1 -timeout 2400s && rm zarf-init-$(ARCH).tar.zst
