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

# remove all zarf packages recursively
remove-packages:
	find . -type f -name 'zarf-package*' -delete

# usage: make test OS=ubuntu
test:
	vagrant destroy -f
	vagrant up --no-color ${OS}
	echo -e "\n\n\n\033[1;93m  ✅ BUILD COMPLETE.  To access this environment, run \"vagrant ssh ${OS}\"\n\n\n"

test-close:
	vagrant destroy -f

package:
	$(ZARF_BIN) package create --confirm
	mv zarf*.tar.zst build

	cd build && sha256sum -b zarf* > zarf.sha256
	ls -lh build

build-cli:
	rm -fr build
	cd cli && $(MAKE) build
	cd cli && $(MAKE) build-mac

build-test: build-cli package

ci-release: package

# automatically package all example directories and add the tarballs to the build directory
package-examples:
	cd examples && $(MAKE) package-examples
