.SILENT:
.DEFAULT_GOAL := ci

SHELL := /bin/bash

SRCDIR := $(patsubst %/,%,$(dir $(abspath $(lastword $(MAKEFILE_LIST)))))
GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)
LINT_DIRTY ?= false
VERSION ?= $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null | tr '[:upper:]' '[:lower:]' || echo "unknown")

DEPS_UPDATE ?= false
deps:
	@echo "+++ $@ +++"

	cd $(SRCDIR) && go mod tidy && go mod download
	cd $(SRCDIR)/cmd/gguf-parser && go mod tidy && go mod download

	if [[ "$(DEPS_UPDATE)" == "true" ]]; then \
		cd $(SRCDIR) && go get -u -v ./...; \
		cd $(SRCDIR)/cmd/gguf-parser && go get -u -v ./...; \
	fi

	@echo "--- $@ ---"

generate:
	@echo "+++ $@ +++"

	cd $(SRCDIR) && go generate ./...
	cd $(SRCDIR)/cmd/gguf-parser && go generate ./...

	@echo "--- $@ ---"

lint:
	@echo "+++ $@ +++"

	[[ -d "$(SRCDIR)/.sbin" ]] || mkdir -p "$(SRCDIR)/.sbin"

	[[ -f "$(SRCDIR)/.sbin/goimports-reviser" ]] || \
		curl --retry 3 --retry-all-errors --retry-delay 3 -sSfL "https://github.com/incu6us/goimports-reviser/releases/download/v3.8.2/goimports-reviser_3.8.2_$(GOOS)_$(GOARCH).tar.gz" \
		| tar -zxvf - --directory "$(SRCDIR)/.sbin" --no-same-owner --exclude ./LICENSE --exclude ./README.md && chmod +x "$(SRCDIR)/.sbin/goimports-reviser"
	cd $(SRCDIR) && \
		go list -f "{{.Dir}}" ./... | xargs -I {} find {} -maxdepth 1 -type f -name '*.go' ! -name 'gen.*' ! -name 'zz_generated.*' \
		| xargs -I {} "$(SRCDIR)/.sbin/goimports-reviser" -use-cache -imports-order=std,general,company,project,blanked,dotted -output=file {} 1>/dev/null 2>&1
	cd $(SRCDIR)/cmd/gguf-parser && \
		go list -f "{{.Dir}}" ./... | xargs -I {} find {} -maxdepth 1 -type f -name '*.go' ! -name 'gen.*' ! -name 'zz_generated.*' \
		| xargs -I {} "$(SRCDIR)/.sbin/goimports-reviser" -use-cache -imports-order=std,general,company,project,blanked,dotted -output=file {} 1>/dev/null 2>&1

	[[ -f "$(SRCDIR)/.sbin/golangci-lint" ]] || \
		curl --retry 3 --retry-all-errors --retry-delay 3 -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
		| sh -s -- -b "$(SRCDIR)/.sbin" "v1.63.4"
	cd $(SRCDIR) && \
		"$(SRCDIR)/.sbin/golangci-lint" run --fix ./...
	cd $(SRCDIR)/cmd/gguf-parser && \
		"$(SRCDIR)/.sbin/golangci-lint" run --fix ./...

	if [[ "$(LINT_DIRTY)" == "true" ]]; then \
		if [[ -n $$(git status --porcelain) ]]; then \
			echo "Code tree is dirty."; \
			git diff --exit-code; \
		fi; \
	fi

	@echo "--- $@ ---"

test:
	@echo "+++ $@ +++"

	go test -v -failfast -race -cover -timeout=30m $(SRCDIR)/...

	@echo "--- $@ ---"

benchmark:
	@echo "+++ $@ +++"

	go test -v -failfast -run="^Benchmark[A-Z]+" -bench=. -benchmem -timeout=30m $(SRCDIR)/...

	@echo "--- $@ ---"

gguf-parser:
	[[ -d "$(SRCDIR)/.dist" ]] || mkdir -p "$(SRCDIR)/.dist"

	cd "$(SRCDIR)/cmd/gguf-parser" && for os in darwin linux windows; do \
  		tags="netgo"; \
  		if [[ $$os == "windows" ]]; then \
		  suffix=".exe"; \
		  tags="netcgo"; \
		else \
		  suffix=""; \
		fi; \
		for arch in amd64 arm64; do \
		  	echo "Building gguf-parser for $$os-$$arch $(VERSION)"; \
			GOOS="$$os" GOARCH="$$arch" CGO_ENABLED=1 go build \
				-trimpath \
				-ldflags="-w -s -X main.Version=$(VERSION)" \
				-tags="urfave_cli_no_docs $$tags" \
				-o $(SRCDIR)/.dist/gguf-parser-$$os-$$arch$$suffix; \
		done; \
		if [[ $$os == "darwin" ]]; then \
		  [[ -d "$(SRCDIR)/.sbin" ]] || mkdir -p "$(SRCDIR)/.sbin"; \
		  [[ -f "$(SRCDIR)/.sbin/lipo" ]] || \
			GOBIN="$(SRCDIR)/.sbin" go install github.com/konoui/lipo@v0.9.2; \
		  	"$(SRCDIR)/.sbin/lipo" -create -output $(SRCDIR)/.dist/gguf-parser-darwin-universal $(SRCDIR)/.dist/gguf-parser-darwin-amd64 $(SRCDIR)/.dist/gguf-parser-darwin-arm64; \
		fi;\
		if [[ $$os == "$(GOOS)" ]] && [[ $$arch == "$(GOARCH)" ]]; then \
			cp -rf $(SRCDIR)/.dist/gguf-parser-$$os-$$arch$$suffix $(SRCDIR)/.dist/gguf-parser$$suffix; \
		fi; \
	done

build: gguf-parser

PACKAGE_PUBLISH ?= false
PACKAGE_REGISTRY ?= "gpustack"
PACKAGE_IMAGE ?= "gguf-parser"
package: build
	@echo "+++ $@ +++"

	if [[ -z $$(command -v docker) ]]; then \
  		echo "Docker is not installed."; \
		exit 1; \
	fi; \
	platform="linux/amd64,linux/arm64"; \
	image="$(PACKAGE_IMAGE):$(VERSION)"; \
	if [[ -n "$(PACKAGE_REGISTRY)" ]]; then \
		image="$(PACKAGE_REGISTRY)/$$image"; \
	fi; \
	if [[ "$(PACKAGE_PUBLISH)" == "true" ]]; then \
	  	if [[ -z $$(docker buildx inspect --builder "gguf-parser") ]]; then \
      		docker run --rm --privileged tonistiigi/binfmt:qemu-v9.2.2 --install $$platform; \
      		docker buildx create --name "gguf-parser" --driver "docker-container" --buildkitd-flags "--allow-insecure-entitlement security.insecure --allow-insecure-entitlement network.host" --bootstrap; \
      	fi; \
		docker buildx build --progress=plain --platform=$$platform --builder="gguf-parser" --output="type=image,name=$$image,push=true" "$(SRCDIR)"; \
	else \
	  	platform="linux/$(GOARCH)"; \
  		docker buildx build --progress=plain --platform=$$platform --output="type=docker,name=$$image" "$(SRCDIR)"; \
	fi

	@echo "--- $@ ---"

ci: deps generate lint test build
