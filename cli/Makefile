MAKEFLAGS += --silent

# Tests have been purged for now due to large refactor deleted all tested code
# deps:
# 	go get gotest.tools/gotestsum 
# 	go mod download

# test: deps
# 	gotestsum

build:
	mkdir -p ../build
	GOOS=linux GOARCH=amd64 go build -o ../build/zarf main.go