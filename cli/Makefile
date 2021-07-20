# Tests have been purged for now due to large refactor deleted all tested code
# deps:
# 	go get gotest.tools/gotestsum 
# 	go mod download

# test: deps
# 	gotestsum

build:
	mkdir -p ../build
	GOOS=linux GOARCH=amd64 go build -o ../build/zarf main.go

build-mac:
	mkdir -p ../build
	GOOS=darwin GOARCH=arm64 go build -o ../build/zarf-mac-apple main.go
	GOOS=darwin GOARCH=amd64 go build -o ../build/zarf-mac-intel main.go
	