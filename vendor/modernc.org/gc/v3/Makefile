.PHONY:	all clean edit editor test test2 back report report2 parser2 benchmarks benchmarks2 mem memgo race nreport build_all_targets

all:

build_all_targets:
	GOOS=darwin GOARCH=amd64 go test -c -o /dev/null
	GOOS=darwin GOARCH=arm64 go test -c -o /dev/null
	GOOS=freebsd GOARCH=386 go test -c -o /dev/null
	GOOS=freebsd GOARCH=amd64 go test -c -o /dev/null
	GOOS=freebsd GOARCH=arm go test -c -o /dev/null
	GOOS=freebsd GOARCH=arm64 go test -c -o /dev/null
	GOOS=illumos GOARCH=amd64 go test -c -o /dev/null
	GOOS=linux GOARCH=386 go test -c -o /dev/null
	GOOS=linux GOARCH=amd64 go test -c -o /dev/null
	GOOS=linux GOARCH=arm go test -c -o /dev/null
	GOOS=linux GOARCH=arm64 go test -c -o /dev/null
	GOOS=linux GOARCH=loong64 go test -c -o /dev/null
	GOOS=linux GOARCH=ppc64le go test -c -o /dev/null
	GOOS=linux GOARCH=riscv64 go test -c -o /dev/null
	GOOS=linux GOARCH=s390x go test -c -o /dev/null
	GOOS=netbsd GOARCH=386 go test -c -o /dev/null
	GOOS=netbsd GOARCH=amd64 go test -c -o /dev/null
	GOOS=netbsd GOARCH=arm go test -c -o /dev/null
	GOOS=openbsd GOARCH=386 go test -c -o /dev/null
	GOOS=openbsd GOARCH=amd64 go test -c -o /dev/null
	GOOS=openbsd GOARCH=arm64 go test -c -o /dev/null
	GOOS=windows GOARCH=386 go test -c -o /dev/null
	GOOS=windows GOARCH=amd64 go test -c -o /dev/null
	GOOS=windows GOARCH=arm64 go test -c -o /dev/null

clean:
	rm -f cpu.test mem.test *.out
	go clean

edit:
	@touch log
	@if [ -f "Session.vim" ]; then gvim -S & else gvim -p Makefile go.mod builder.json all_test.go gc.go & fi

editor:
	gofmt -l -s -w *.go
	go test -c -o /dev/null

race:
	go test -v -failfast -heap -race
	@git diff testdata/ || true
	@git status

test:
	go test -v -failfast -trctodo -exterr -heap
	@git diff testdata/ || true
	@git status

test2:
	go test -v -failfast -trctodo -exterr -src $$HOME/src
	@git diff testdata/ || true
	@git status

parser2:
	go test -v -failfast -run TestParser -src $$HOME/src
	@git diff testdata/ || true
	@git status

back:
	go test -v -failfast -noback
	@git diff testdata/ || true
	@git status

nreport:
	touch log-nreport
	cp log-nreport log-nreport0
	go test -v -failfast -run TestParser -heap -nreport
	@git diff testdata/ || true
	@git status

report:
	go test -v -failfast -run TestParser -report
	@git diff testdata/ || true
	@git status

report2:
	go test -v -failfast -run TestParser -src $$HOME/src -report
	@git diff testdata/ || true
	@git status


benchmarks:
	go test -v -run @ -bench .

benchmarks2:
	go test -v -run @ -bench . -bsrc $$HOME/src

mem:
	go test -run @ -bench BenchmarkParser -memprofile mem.out
	go tool pprof --lines --alloc_space *.test mem.out

memgo:
	go test -run @ -bench BenchmarkGoParser -memprofile mem.out
	go tool pprof --lines --alloc_space *.test mem.out
