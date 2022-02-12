# Tests have been purged for now due to large refactor deleted all tested code
# deps:
# 	go get gotest.tools/gotestsum
# 	go mod download

# test: deps
# 	gotestsum

CLI_VERSION := $(if $(shell git describe --tags), $(shell git describe --tags), "UnknownVersion")

build:
	mkdir -p ../build
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X 'github.com/defenseunicorns/zarf/cli/config.CLIVersion=$(CLI_VERSION)'" -o ../build/zarf main.go
build-mac:
	mkdir -p ../build
	GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w -X 'github.com/defenseunicorns/zarf/cli/config.CLIVersion=$(CLI_VERSION)'" -o ../build/zarf-mac-apple main.go
	GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w -X 'github.com/defenseunicorns/zarf/cli/config.CLIVersion=$(CLI_VERSION)'" -o ../build/zarf-mac-intel main.go
