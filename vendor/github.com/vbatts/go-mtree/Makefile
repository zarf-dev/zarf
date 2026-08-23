
BUILD := gomtree
BUILDPATH := github.com/vbatts/go-mtree/cmd/gomtree
CWD := $(shell pwd)
SOURCE_FILES := $(shell find . -type f -name "*.go")
CLEAN_FILES := *~
TAGS :=
ARCHES := linux,386 linux,amd64 linux,arm linux,arm64 linux,mips64 linux,riscv64 openbsd,amd64 windows,amd64 windows,arm64 darwin,amd64 darwin,arm64
GO_VER := go1.14

default: build validation

.PHONY: validation
validation: .test .vet .cli.test

.PHONY: validation.tags
validation.tags: .test.tags .vet.tags .cli.test .staticcheck

.PHONY: gocyclo
gocyclo: .gocyclo

CLEAN_FILES += .gocyclo

.gocyclo:
	gocyclo -avg -over 15 -ignore 'vendor/*' . && touch $@

.PHONY: staticcheck
staticcheck: .staticcheck

CLEAN_FILES += .staticcheck

.staticcheck:
	staticcheck . && touch $@

.PHONY: test
test: .test

CLEAN_FILES += .test .test.tags
NO_VENDOR_DIR := $(shell find . -type f -name '*.go' ! -path './vendor*' ! -path './.git*' ! -path './.vscode*' -exec dirname "{}" \; | sort -u)

.test: $(SOURCE_FILES)
	go test -v $(NO_VENDOR_DIR) && touch $@

.test.tags: $(SOURCE_FILES)
	set -e ; for tag in $(TAGS) ; do go test -tags $$tag -v $(NO_VENDOR_DIR) ; done && touch $@

.PHONY: lint
lint: .lint

CLEAN_FILES += .lint

.lint: $(SOURCE_FILES)
	set -e ; golangci-lint run && touch $@

.PHONY: vet
vet: .vet .vet.tags

CLEAN_FILES += .vet .vet.tags

.vet: $(SOURCE_FILES)
	go vet $(NO_VENDOR_DIR) && touch $@

.vet.tags: $(SOURCE_FILES)
	set -e ; for tag in $(TAGS) ; do go vet -tags $$tag -v $(NO_VENDOR_DIR) ; done && touch $@

.PHONY: cli.test
cli.test: .cli.test

CLEAN_FILES += .cli.test .cli.test.tags

.cli.test: $(BUILD) $(wildcard ./test/cli/*.sh)
	@go run ./test/cli-test/main.go ./test/cli/*.sh && touch $@

.cli.test.tags: $(BUILD) $(wildcard ./test/cli/*.sh)
	@set -e ; for tag in $(TAGS) ; do go run -tags $$tag ./test/cli-test/main.go ./test/cli/*.sh ; done && touch $@

.PHONY: build
build: $(BUILD)

$(BUILD): $(SOURCE_FILES)
	go build -ldflags="-X 'main.Version=$(shell git describe --always --dirty)'" -mod=vendor -o $(BUILD) $(BUILDPATH)

install.tools:
	@go install github.com/fatih/color@latest ; \
	go install github.com/fzipp/gocyclo/cmd/gocyclo@latest ; \
	go install honnef.co/go/tools/cmd/staticcheck@latest ; \
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

./bin:
	mkdir -p $@

CLEAN_FILES += bin

build.arches: ./bin
	@set -e ;\
	for pair in $(ARCHES); do \
	p=$$(echo $$pair | cut -d , -f 1);\
	a=$$(echo $$pair | cut -d , -f 2);\
	echo "Building $$p/$$a ...";\
	GOOS=$$p GOARCH=$$a go build -mod=vendor -o ./bin/gomtree.$$p.$$a $(BUILDPATH) ;\
	done ;\
	cd bin ;\
	sha1sum gomtree.* > SUMS ;\
	sha512sum gomtree.* >> SUMS ;\
	cd -

clean:
	rm -rf $(BUILD) $(CLEAN_FILES)

