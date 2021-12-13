.DEFAULT_GOAL := help

.PHONY: help
help: ## Show a list of all targets
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
	| sed -n 's/^\(.*\): \(.*\)##\(.*\)/\1:\3/p' \
	| column -t -s ":"

.PHONY: all
all: ## Download the latest version of Zarf, build the deploy package, and start a VM with Vagrant
	@cd .. && $(MAKE) clean fetch-release package-example-postgres-operator vm-init

.PHONY: all-dev
all-dev: ## Same as 'default', but build Zarf rather than downloading it
	@cd .. && $(MAKE) clean build-release package-example-postgres-operator vm-init

.PHONY: vm-init
vm-init: ## Bring up the VM
	@cd .. && $(MAKE) vm-init

.PHONY: vm-destroy
vm-destroy: ## Destroy the VM
	@cd .. && $(MAKE) vm-destroy
