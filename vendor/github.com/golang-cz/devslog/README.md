# 🧻 devslog - Go [slog.Handler](https://pkg.go.dev/log/slog#Handler) for development

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://github.com/golang-cz/devslog/blob/master/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/golang-cz/devslog)](https://goreportcard.com/report/github.com/golang-cz/devslog)
[![Go Reference](https://pkg.go.dev/badge/github.com/golang-cz/devslog.svg)](https://pkg.go.dev/github.com/golang-cz/devslog)

`devslog` is a zero dependency structured logging handler for Go's [`log/slog`](https://pkg.go.dev/log/slog) package with pretty and colorful output for developers.

### Devslog output

![image](https://github.com/golang-cz/devslog/assets/17728576/cfdc1634-16fe-4dd0-a643-21bf519cd4fe)

#### Compared to

`TextHandler`
![image](https://github.com/golang-cz/devslog/assets/17728576/49aab1c0-93ba-409d-8637-a96eeeaaf0e1)

`JSONHandler`
![image](https://github.com/golang-cz/devslog/assets/17728576/775af693-2f96-47e8-9190-5ead77b41a27)

## Install

```
go get github.com/golang-cz/devslog@latest
```

## Examples

### Logger without options

```go
logger := slog.New(devslog.NewHandler(os.Stdout, nil))

// optional: set global logger
slog.SetDefault(logger)
```

### Logger with custom options

```go
// new logger with options
opts := &devslog.Options{
	MaxSlicePrintSize: 4,
	SortKeys:          true,
	TimeFormat:        "[04:05]",
	NewLineAfterLog:   true,
	DebugColor:        devslog.Magenta,
	StringerFormatter: true,
}

logger := slog.New(devslog.NewHandler(os.Stdout, opts))

// optional: set global logger
slog.SetDefault(logger)
```

### Logger with default slog options

Handler accepts default [slog.HandlerOptions](https://pkg.go.dev/golang.org/x/exp/slog#HandlerOptions)

```go
// slog.HandlerOptions
slogOpts := &slog.HandlerOptions{
	AddSource:   true,
	Level:       slog.LevelDebug,
}

// new logger with options
opts := &devslog.Options{
	HandlerOptions:    slogOpts,
	MaxSlicePrintSize: 4,
	SortKeys:          true,
	NewLineAfterLog:   true,
	StringerFormatter: true,
}

logger := slog.New(devslog.NewHandler(os.Stdout, opts))

// optional: set global logger
slog.SetDefault(logger)
```

### Example usage

```go
slogOpts := &slog.HandlerOptions{
	AddSource: true,
	Level:     slog.LevelDebug,
}

var logger *slog.Logger
if production {
	logger = slog.New(slog.NewJSONHandler(os.Stdout, slogOpts))
} else {
	opts := &devslog.Options{
		HandlerOptions:    slogOpts,
		MaxSlicePrintSize: 10,
		SortKeys:          true,
		NewLineAfterLog:   true,
		StringerFormatter: true,
	}

	logger = slog.New(devslog.NewHandler(os.Stdout, opts))
}

// optional: set global logger
slog.SetDefault(logger)
```

## Options

| Parameter           | Description                                                    | Default        | Value                |
| ------------------- | -------------------------------------------------------------- | -------------- | -------------------- |
| MaxSlicePrintSize   | Specifies the maximum number of elements to print for a slice. | 50             | uint                 |
| SortKeys            | Determines if attributes should be sorted by keys.             | false          | bool                 |
| TimeFormat          | Time format for timestamp.                                     | "[15:04:05]"   | string               |
| NewLineAfterLog     | Add blank line after each log                                  | false          | bool                 |
| StringIndentation   | Indent \n in strings                                           | false          | bool                 |
| DebugColor          | Color for Debug level                                          | devslog.Blue   | devslog.Color (uint) |
| InfoColor           | Color for Info level                                           | devslog.Green  | devslog.Color (uint) |
| WarnColor           | Color for Warn level                                           | devslog.Yellow | devslog.Color (uint) |
| ErrorColor          | Color for Error level                                          | devslog.Red    | devslog.Color (uint) |
| MaxErrorStackTrace  | Max stack trace frames for errors                              | 0              | uint                 |
| StringerFormatter   | Use Stringer interface for formatting                          | false          | bool                 |
| NoColor             | Disable coloring                                               | false          | bool                 |
| SameSourceInfoColor | Keep same color for whole source info                          | false          | bool                 |
