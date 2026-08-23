.PHONY: fmt check-fmt lint vet test

test:
	@go test -race ./...

retest:
	@rerun -watch . ../devslog -run sh -c 'make test-tparse'

test-coverage:
	@go test -race -cover -coverprofile=coverage.txt ./... -json

test-coverage-inspect: test-coverage
	@go tool cover -html=coverage.txt

test-tparse:
	@go test -race ./... -json | tparse -all

test-coverage-tparse:
	@go test -race -cover -coverprofile=coverage.txt ./... -json | tparse -all
