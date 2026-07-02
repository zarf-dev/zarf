// Package logger provides a logger abstraction for writing log messages in
// configurable formats to different outputs, such as a console, plain text
// file, or a JSON file.
//
// It is intended for internal use by buildkite-agent only.
package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/buildkite/agent/v3/version"
	"golang.org/x/term"
)

const (
	nocolor   = "0"
	red       = "31"
	green     = "38;5;48"
	yellow    = "33"
	gray      = "38;5;251"
	graybold  = "1;38;5;251"
	lightgray = "38;5;243"
	cyan      = "1;36"
)

const (
	DateFormat = "2006-01-02 15:04:05"
)

var (
	mutex         = sync.Mutex{}
	windowsColors bool
)

type Logger interface {
	Debug(format string, v ...any)
	Error(format string, v ...any)
	Fatal(format string, v ...any)
	Notice(format string, v ...any)
	Warn(format string, v ...any)
	Info(format string, v ...any)

	WithFields(fields ...Field) Logger
	SetLevel(level Level)
	Level() Level
}

type ConsoleLogger struct {
	level   Level
	exitFn  func(int)
	fields  Fields
	printer Printer
}

func NewConsoleLogger(printer Printer, exitFn func(int)) Logger {
	return &ConsoleLogger{
		level:   DEBUG,
		fields:  Fields{},
		printer: printer,
		exitFn:  exitFn,
	}
}

// WithFields returns a copy of the logger with the provided fields
func (l *ConsoleLogger) WithFields(fields ...Field) Logger {
	clone := *l
	clone.fields.Add(fields...)
	return &clone
}

// SetLevel sets the level in the logger
func (l *ConsoleLogger) SetLevel(level Level) {
	l.level = level
}

func (l *ConsoleLogger) Debug(format string, v ...any) {
	if l.level == DEBUG {
		debugFields := make(Fields, len(l.fields))
		copy(debugFields, l.fields)
		debugFields.Add(StringField("agent_version", version.FullVersion()))
		l.printer.Print(DEBUG, fmt.Sprintf(format, v...), debugFields)
	}
}

func (l *ConsoleLogger) Error(format string, v ...any) {
	l.printer.Print(ERROR, fmt.Sprintf(format, v...), l.fields)
}

func (l *ConsoleLogger) Fatal(format string, v ...any) {
	l.printer.Print(FATAL, fmt.Sprintf(format, v...), l.fields)
	l.exitFn(1)
}

func (l *ConsoleLogger) Notice(format string, v ...any) {
	if l.level <= NOTICE {
		l.printer.Print(NOTICE, fmt.Sprintf(format, v...), l.fields)
	}
}

func (l *ConsoleLogger) Info(format string, v ...any) {
	if l.level <= INFO {
		l.printer.Print(INFO, fmt.Sprintf(format, v...), l.fields)
	}
}

func (l *ConsoleLogger) Warn(format string, v ...any) {
	if l.level <= WARN {
		l.printer.Print(WARN, fmt.Sprintf(format, v...), l.fields)
	}
}

func (l *ConsoleLogger) Level() Level {
	return l.level
}

type Printer interface {
	Print(level Level, msg string, fields Fields)
}

type TextPrinter struct {
	Colors bool
	Writer io.Writer

	IsPrefixFn  func(Field) bool
	IsVisibleFn func(Field) bool
}

func NewTextPrinter(w io.Writer) *TextPrinter {
	return &TextPrinter{
		Writer: w,
		Colors: ColorsSupported(),
	}
}

func (l *TextPrinter) Print(level Level, msg string, fields Fields) {
	now := time.Now().Format(DateFormat)

	var line string
	var prefix string
	var fieldStrs []string

	if l.IsPrefixFn != nil {
		for _, f := range fields {
			// Skip invisible fields
			if l.IsVisibleFn != nil && !l.IsVisibleFn(f) {
				continue
			}
			// Allow some fields to be shown as prefixes
			if l.IsPrefixFn(f) {
				prefix += f.String()
			}
		}
	}

	if l.Colors {
		levelColor := green
		messageColor := nocolor
		fieldColor := graybold

		switch level {
		case DEBUG:
			levelColor = gray
			messageColor = gray
		case NOTICE:
			levelColor = cyan
		case WARN:
			levelColor = yellow
		case ERROR:
			levelColor = red
		case FATAL:
			levelColor = red
			messageColor = red
		}

		if prefix != "" {
			line = fmt.Sprintf("\x1b[%sm%s %-6s\x1b[0m \x1b[%sm%s\x1b[0m \x1b[%sm%s\x1b[0m",
				levelColor, now, level, lightgray, prefix, messageColor, msg)
		} else {
			line = fmt.Sprintf("\x1b[%sm%s %-6s\x1b[0m \x1b[%sm%s\x1b[0m",
				levelColor, now, level, messageColor, msg)
		}

		for _, field := range fields {
			if l.IsVisibleFn != nil && !l.IsVisibleFn(field) {
				continue
			}
			if l.IsPrefixFn != nil && l.IsPrefixFn(field) {
				continue
			}
			fieldStrs = append(fieldStrs, fmt.Sprintf("\x1b[%sm%s=\x1b[0m\x1b[%sm%s\x1b[0m",
				fieldColor, field.Key(), messageColor, field.String()))
		}
	} else {
		if prefix != "" {
			line = fmt.Sprintf("%s %-6s %s %s", now, level, prefix, msg)
		} else {
			line = fmt.Sprintf("%s %-6s %s", now, level, msg)
		}

		for _, field := range fields {
			if l.IsVisibleFn != nil && !l.IsVisibleFn(field) {
				continue
			}
			if l.IsPrefixFn != nil && l.IsPrefixFn(field) {
				continue
			}
			fieldStrs = append(fieldStrs, fmt.Sprintf("%s=%s", field.Key(), field.String()))
		}
	}

	// Make sure we're only outputting a line one at a time
	mutex.Lock()
	fmt.Fprint(l.Writer, line)
	if len(fields) > 0 {
		fmt.Fprintf(l.Writer, " %s", strings.Join(fieldStrs, " "))
	}
	fmt.Fprint(l.Writer, "\n")
	mutex.Unlock()
}

func ColorsSupported() bool {
	// Color support for windows is set in init
	if runtime.GOOS == "windows" && !windowsColors {
		return false
	}

	// Colors can only be shown if STDOUT is a terminal
	return term.IsTerminal(int(os.Stdout.Fd()))
}

type JSONPrinter struct {
	Writer io.Writer
}

func NewJSONPrinter(w io.Writer) *JSONPrinter {
	return &JSONPrinter{
		Writer: w,
	}
}

func (p *JSONPrinter) Print(level Level, msg string, fields Fields) {
	var b strings.Builder

	b.WriteString(fmt.Sprintf(`"ts":%q,`, time.Now().Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf(`"level":%q,`, level.String()))

	// Serialize msg to JSON so we're not producing invalid JSON
	jsonMsg, err := json.Marshal(msg)
	if err != nil {
		jsonMsg = []byte(`"error marshaling message"`)
	}
	b.WriteString(fmt.Sprintf(`"msg":%s,`, jsonMsg))

	for _, field := range fields {
		b.WriteString(fmt.Sprintf("%q:%q,", field.Key(), field.String()))
	}

	// Make sure we're only outputting a line one at a time
	mutex.Lock()
	fmt.Fprintf(p.Writer, "{%s}\n", strings.TrimSuffix(b.String(), ","))
	mutex.Unlock()
}

var Discard = &ConsoleLogger{
	printer: &TextPrinter{
		Writer: io.Discard,
	},
}

// TestPrinter is a log printer than calls the Logf method of a [testing.T]
// or [testing.B].
type TestPrinter struct {
	tb testing.TB
}

func NewTestPrinter(tb testing.TB) TestPrinter {
	return TestPrinter{tb: tb}
}

func (tp TestPrinter) Print(level Level, msg string, fields Fields) {
	now := time.Now().Format(DateFormat)
	tp.tb.Logf("%s %s %s %v", now, level, msg, fields)
}
