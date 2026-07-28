# console-slog

[![Go Reference](https://pkg.go.dev/badge/github.com/phsym/console-slog.svg)](https://pkg.go.dev/github.com/phsym/console-slog) [![license](http://img.shields.io/badge/license-MIT-red.svg?style=flat)](https://raw.githubusercontent.com/phsym/console-slog/master/LICENSE) [![Build](https://github.com/phsym/console-slog/actions/workflows/go.yml/badge.svg?branch=main)](https://github.com/phsym/slog-console/actions/workflows/go.yml) [![codecov](https://codecov.io/gh/phsym/console-slog/graph/badge.svg?token=ZIJT9L79QP)](https://codecov.io/gh/phsym/console-slog) [![Go Report Card](https://goreportcard.com/badge/github.com/phsym/console-slog)](https://goreportcard.com/report/github.com/phsym/console-slog)

A handler for slog that prints colorized logs, similar to zerolog's console writer output without sacrificing performances.

## Installation
```bash
go get github.com/phsym/console-slog@latest
```

## Example
```go
package main

import (
	"errors"
	"log/slog"
	"os"

	"github.com/phsym/console-slog"
)

func main() {
	logger := slog.New(
		console.NewHandler(os.Stderr, &console.HandlerOptions{Level: slog.LevelDebug}),
	)
	slog.SetDefault(logger)
	slog.Info("Hello world!", "foo", "bar")
	slog.Debug("Debug message")
	slog.Warn("Warning message")
	slog.Error("Error message", "err", errors.New("the error"))

	logger = logger.With("foo", "bar").
		WithGroup("the-group").
		With("bar", "baz")

	logger.Info("group info", "attr", "value")
}
```

![output](./doc/img/output.png)

When setting `console.HandlerOptions.AddSource` to `true`:
```go
console.NewHandler(os.Stderr, &console.HandlerOptions{Level: slog.LevelDebug, AddSource: true})
```
![output-with-source](./doc/img/output-with-source.png)

## Performances
See [benchmark file](./bench_test.go) for details.

The handler itself performs quite well compared to std-lib's handlers. It does no allocation:
```
goos: linux
goarch: amd64
pkg: github.com/phsym/console-slog
cpu: Intel(R) Core(TM) i5-6300U CPU @ 2.40GHz
BenchmarkHandlers/dummy-4               128931026            8.732 ns/op               0 B/op          0 allocs/op
BenchmarkHandlers/console-4               849837              1294 ns/op               0 B/op          0 allocs/op
BenchmarkHandlers/std-text-4              542583              2097 ns/op               4 B/op          2 allocs/op
BenchmarkHandlers/std-json-4              583784              1911 ns/op             120 B/op          3 allocs/op
```

However, the go 1.21.0 `slog.Logger` adds some overhead:
```
goos: linux
goarch: amd64
pkg: github.com/phsym/console-slog
cpu: Intel(R) Core(TM) i5-6300U CPU @ 2.40GHz
BenchmarkLoggers/dummy-4                 1239873             893.2 ns/op             128 B/op          1 allocs/op
BenchmarkLoggers/console-4                483354              2338 ns/op             128 B/op          1 allocs/op
BenchmarkLoggers/std-text-4               368828              3141 ns/op             132 B/op          3 allocs/op
BenchmarkLoggers/std-json-4               393322              2909 ns/op             248 B/op          4 allocs/op
```