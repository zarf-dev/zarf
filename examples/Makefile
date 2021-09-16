# Figure out which Zarf binary we should use based on the operating system we are on
ZARF_BIN := ../../build/zarf
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

package-examples:
#	cd appliance               && $(ZARF_BIN) package create --confirm && mv zarf*.tar.zst ../../build
#	cd data-injection          && $(ZARF_BIN) package create --confirm && mv zarf*.tar     ../../build
#	cd game                    && $(ZARF_BIN) package create --confirm && mv zarf*.tar.zst ../../build
#	cd single-big-bang-package && $(ZARF_BIN) package create --confirm && mv zarf*.tar.zst ../../build
#	cd tiny-kafka              && $(ZARF_BIN) package create --confirm && mv zarf*.tar.zst ../../build
	cd postgres-operator       && $(ZARF_BIN) package create --confirm && mv zarf*.tar.zst ../../build
