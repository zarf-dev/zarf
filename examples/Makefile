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

# Download zarf, build all packages and launch a basic VM with the assets
.PHONY: all
all: fetch-release package-examples vm-init

# Same as target 'all', but build the binaries using the current codebase rather than downloading the latest version from the internet
.PHONY: all-dev
all-dev: build-release package-examples vm-init

# Clean the sync dir
.PHONY: clean
clean:
	@rm -fr sync && mkdir -p sync

# Grab the latest release as an alternative to needing to build the binaries
.PHONY: fetch-release
fetch-release: clean
	@# This probably isn't the cleanest way to get a release, but since we're moving to github, not worth adding the code until post-migration
	@curl -fL "https://zarf-public.s3-us-gov-west-1.amazonaws.com/release/$$(git describe --tags --abbrev=0)/{zarf,zarf-mac-intel,zarf-mac-apple,zarf-init.tar.zst}" -o "sync/#1"
	@chmod +x sync/*

# Build the binaries as an alternative to downloading the latest release
.PHONY: build-release
build-release: clean
	@cd .. && $(MAKE) build-test
	@cp -R ../build/* sync

# Stripped-down vagrant box to reduce friction for basic user testing
# Note the need to perform disk resizing for some examples
.PHONY: vm-init
vm-init: vm-destroy
	@VAGRANT_EXPERIMENTAL="disks" vagrant up --no-color
	@echo -e "\n\n\n\033[1;93m  ✅ VM READY.  Logging in now, run \"sudo su\" once the prompt appears.\n\n\n\033[0m"
	@vagrant ssh

# Cleanup plz
.PHONY: vm-destroy
vm-destroy:
	@vagrant destroy -f

# Create zarf packages from all examples
.PHONY: package-examples
package-examples: package-example-big-bang package-example-appliance package-example-data-injection package-example-game package-example-single-big-bang-package package-example-tiny-kafka package-example-postgres-operator

.PHONY: package-example-big-bang
package-example-big-bang:
	cd big-bang && kustomize build template/bigbang > manifests/bigbang_generated.yaml && kustomize build template/flux > manifests/flux_generated.yaml && $(ZARF_BIN) package create --confirm && mv zarf-package-* ../sync/

.PHONY: package-example-appliance
package-example-appliance:
	cd appliance && $(ZARF_BIN) package create --confirm && mv zarf-package-* ../sync/

.PHONY: package-example-data-injection
package-example-data-injection:
	cd data-injection && $(ZARF_BIN) package create --confirm && mv zarf-package-* ../sync/

.PHONY: package-example-game
package-example-game:
	cd game && $(ZARF_BIN) package create --confirm && mv zarf-package-* ../sync/

.PHONY: package-example-single-big-bang-package
package-example-single-big-bang-package:
	cd single-big-bang-package && $(ZARF_BIN) package create --confirm && mv zarf-package-* ../sync/

.PHONY: package-example-tiny-kafka
package-example-tiny-kafka:
	cd tiny-kafka && $(ZARF_BIN) package create --confirm && mv zarf-package-* ../sync/

.PHONY: package-example-postgres-operator
package-example-postgres-operator:
	cd postgres-operator && $(ZARF_BIN) package create --confirm && mv zarf-package-* ../sync/
