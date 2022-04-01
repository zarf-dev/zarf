# Tests have been purged for now due to large refactor deleted all tested code
# deps:
# 	go get gotest.tools/gotestsum
# 	go mod download

# test: deps
# 	gotestsum

CLI_VERSION := $(if $(shell git describe --tags), $(shell git describe --tags), "UnknownVersion")

build:
	mkdir -p ../build
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X 'github.com/defenseunicorns/zarf/src/config.CLIVersion=$(CLI_VERSION)'" -o ../build/zarf main.go

build-mac:
	mkdir -p ../build
	GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w -X 'github.com/defenseunicorns/zarf/src/config.CLIVersion=$(CLI_VERSION)'" -o ../build/zarf-mac-apple main.go
	GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w -X 'github.com/defenseunicorns/zarf/src/config.CLIVersion=$(CLI_VERSION)'" -o ../build/zarf-mac-intel main.go

build-windows:
	mkdir -p ../build
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -X 'github.com/defenseunicorns/zarf/src/config.CLIVersion=$(CLI_VERSION)'" -o ../build/zarf-windows-amd64.exe main.go
