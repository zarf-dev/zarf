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

build-cli: clean build-cli-linux build-cli-mac ## Build the CLI

init-package: ## Create the zarf init package, macos "brew install coreutils" first
	$(ZARF_BIN) package create --confirm
	mv zarf-init.tar.zst build
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
	cd examples/gitops-data && rm -rf ./.image-cache && ../../$(ZARF_BIN) package create --confirm && mv zarf-package-* ../../build/

.PHONY: test-cloud-e2e-example-game
test-cloud-e2e-example-game: ## Runs the Doom game as an E2E test in the cloud. Requires access to an AWS account. Costs money. Make sure you ran the `build-cli`, `init-package`, and `package-example-game` targets first
	cd test/e2e && go test ./... -run TestE2eExampleGame -v -timeout 1200s

.PHONY: test-cloud-e2e-gitea-and-grafana
test-cloud-e2e-gitea-and-grafana: ## E2E test of Gitea & Grafana. Requires access to an AWS account. Costs money. Make sure you ran the `build-cli` and `init-package` targets first
	cd test/e2e && go test ./... -run TestGiteaAndGrafana -v -timeout 1200s

.PHONY: test-cloud-e2e-gitops
test-cloud-e2e-gitops: package-example-gitops-data ## E2E test of Gitops example. Requires access to an AWS account. Costs money. Make sure you ran the `build-cli` and `init-package` targets first
	cd test/e2e && go test ./... -run TestGitopsExample -v -timeout 1200s

.PHONY: test-cloud-e2e-data-injection
test-cloud-e2e-data-injection: package-example-data-injection ## E2E test of the Data Injection example. Requires access to an AWS account. Costs money. Make sure you ran the `build-cli` and `init-package` targets first
	cd test/e2e && go test ./... -run TestDataInjection -v -timeout 1200s

################ BEGIN Pending removal post-merge
.PHONY: test-cloud-e2e-git-based-helm-chart
test-cloud-e2e-git-based-helm-chart:
	echo done
################ END Pending removal post-merge

.PHONY: test-cloud-e2e-general-cli
test-cloud-e2e-general-cli: ## Runs tests of the CLI that don't need a cluster
	cd test/e2e && go test ./... -run TestGeneralCli -v -timeout 1200s

.PHONY: test-e2e
test-e2e: package-example-game test-cloud-e2e-example-game ## DEPRECATED - to be replaced by individual e2e test targets
