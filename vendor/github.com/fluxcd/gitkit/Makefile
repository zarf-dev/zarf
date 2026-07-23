test:
	go test -v -race -cover .

build:
	go build

all:
	gox -osarch="darwin/amd64 linux/amd64" -output="gitkit_{{.OS}}_{{.Arch}}"

