# Figure out which Zarf binary we should use based on the operating system we are on
ZARF_BIN := zarf
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

PACKAGE_PATH :=  $(shell cd $(name) && pwd)

.PHONY: default 
default: help

.PHONY: package
package:
ifdef name
	@cd $(name) && $(ZARF_BIN) package create --confirm 
	@echo "\nCreated $(name) add the sha and path to your zarf yaml components: " 
	@echo "  - name: $(name)"
	@echo "    files:" 
	@echo "      - source: \"$(PACKAGE_PATH)/zarf-package-$(name).tar.zst\""  
	@echo "        shasum: `cd $(PACKAGE_PATH) && sha256sum zarf-package-$(name).tar.zst | cut -d " " -f1`" 
	@echo "        target: \"/usr/local/bin/zarf-package-$(name).tar.zst\"" 
	@echo "    scripts:" 
	@echo "      after:" 
	@echo "        - \"./zarf package deploy /usr/local/bin/zarf-package-$(name).tar.zst --confirm\""
else
	@echo "Please provide a valid package name. ie: make package name=flux"
endif

.PHONY: help
help:
	@echo "\nAvailable commands: "
	@echo "\n  package     Builds a tar.zst given the name of the component you wish to create."
	@echo "               example: make package name=flux"
	@echo "\n  help        shows this menu."
	@echo