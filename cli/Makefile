MAKEFLAGS += --silent

deps:
	go get gotest.tools/gotestsum 
	go mod download

test: deps
	gotestsum

build-dev: test
	mkdir -p ../build
	GOOS=linux GOARCH=amd64 go build -o ../build/zarf main.go