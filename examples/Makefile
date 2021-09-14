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

# Just grab the latest release instead of having users be forced to build the cli and init packages
fetch-release:
	@rm -fr sync && mkdir -p sync
	@# This probably isn't the cleanest way to get a release, but since we're moving to github, not worth adding the code until post-migration
	@curl -fL "https://zarf-public.s3-us-gov-west-1.amazonaws.com/release/$$(git describe --tags --abbrev=0)/{zarf,zarf-mac-intel,zarf-mac-apple,zarf-init.tar.zst}" -o "sync/#1"
	@chmod +x sync/*

# Stripped-down vagrant box to reduce friction for basic user testing
# Note the need to perform disk resizing for some examples
vm-init:
	@vagrant destroy -f
	@VAGRANT_EXPERIMENTAL="disks" vagrant up --no-color
	@echo -e "\n\n\n\033[1;93m  ✅ VM READY.  Logging in now, run \"sudo su\" once the prompt appears.\n\n\n\033[0m"
	@vagrant ssh

# Cleanup plz
vm-destroy:
	@vagrant destroy -f

# Create zarf packages from all examples
package-examples:
	cd big-bang                && kustomize build template/bigbang > manifests/bigbang_generated.yaml && kustomize build template/flux > manifests/flux_generated.yaml && $(ZARF_BIN) package create --confirm && mv zarf-package-* ../sync/
	cd appliance               && $(ZARF_BIN) package create --confirm && mv zarf-package-* ../sync/
	cd data-injection          && $(ZARF_BIN) package create --confirm && mv zarf-package-* ../sync/
	cd game                    && $(ZARF_BIN) package create --confirm && mv zarf-package-* ../sync/
	cd single-big-bang-package && $(ZARF_BIN) package create --confirm && mv zarf-package-* ../sync/
	cd tiny-kafka              && $(ZARF_BIN) package create --confirm && mv zarf-package-* ../sync/

# Download zarf, build all packages and launch a basic VM with the assets
all: fetch-release package-examples vm-init