PACKAGE_PATH=$(shell cd $(name) && pwd)
SHA_SUM=$(shell sha256sum $(name)/zarf-package-$(name).tar.zst | cut -d " " -f1)
build-package:
ifdef name
	@cd $(name) && zarf package create --confirm 
	@echo "\nCreated $(name) add the sha and path to your zarf yaml components: "
	@echo "  - name: $(name)"
	@echo "    files:"
	@echo "      - source: \"$(PACKAGE_PATH)/zarf-package-$(name).tar.zst\""
	@echo "        shasum: $(SHA_SUM)"
	@echo "        target: /usr/local/bin/zarf-package-$(name).tar.zst"
	@echo "    scripts:"
	@echo "      after:"
	@echo "        - \"./zarf package deploy /usr/local/bin/zarf-package-$(name).tar.zst --confirm\""
else
	@echo "Please provide a valid package name. ie: make build-package name=flux"
endif


help:
	@echo "\nAvailable commands: "
	@echo "\n  build-package     Builds a tar.zst given the name of the component you wish to create."
	@echo "                    example: make build-package name=flux"
	@echo