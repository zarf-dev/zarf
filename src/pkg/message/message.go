// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package message provides a rich set of functions for displaying messages to the user.
package message

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/defenseunicorns/pkg/helpers"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/fatih/color"
	"github.com/pterm/pterm"
	"github.com/sergi/go-diff/diffmatchpatch"
)

// LogLevel is the level of logging to display.
type LogLevel int

const (
	// WarnLevel level. Non-critical entries that deserve eyes.
	WarnLevel LogLevel = iota
	// InfoLevel level. General operational entries about what's going on inside the
	// application.
	InfoLevel
	// DebugLevel level. Usually only enabled when debugging. Very verbose logging.
	DebugLevel
	// TraceLevel level. Designates finer-grained informational events than the Debug.
	TraceLevel

	// TermWidth sets the width of full width elements like progressbars and headers
	TermWidth = 100
)

// NoProgress tracks whether spinner/progress bars show updates.
var NoProgress bool

// RuleLine creates a line of ━ as wide as the terminal
var RuleLine = strings.Repeat("━", TermWidth)

// logLevel holds the pterm compatible log level integer
var logLevel = InfoLevel

// logFile acts as a buffer for logFile generation
var logFile *PausableWriter

// DebugWriter represents a writer interface that writes to message.Debug
type DebugWriter struct{}

func (d *DebugWriter) Write(raw []byte) (int, error) {
	debugPrinter(2, string(raw))
	return len(raw), nil
}

func init() {
	InitializePTerm(os.Stderr)
}

// InitializePTerm sets the default styles and output for pterm.
func InitializePTerm(w io.Writer) {
	pterm.ThemeDefault.SuccessMessageStyle = *pterm.NewStyle(pterm.FgLightGreen)
	// Customize default error.
	pterm.Success.Prefix = pterm.Prefix{
		Text:  " ✔",
		Style: pterm.NewStyle(pterm.FgLightGreen),
	}
	pterm.Error.Prefix = pterm.Prefix{
		Text:  "    ERROR:",
		Style: pterm.NewStyle(pterm.BgLightRed, pterm.FgBlack),
	}
	pterm.Info.Prefix = pterm.Prefix{
		Text: " •",
	}

	pterm.SetDefaultOutput(w)
}

// UseLogFile wraps a given file in a PausableWriter
// and sets it as the log file used by the message package.
func UseLogFile(f *os.File) (*PausableWriter, error) {
	logFile = NewPausableWriter(f)

	return logFile, nil
}

// SetLogLevel sets the log level.
func SetLogLevel(lvl LogLevel) {
	logLevel = lvl
	if logLevel >= DebugLevel {
		pterm.EnableDebugMessages()
	}
}

// GetLogLevel returns the current log level.
func GetLogLevel() LogLevel {
	return logLevel
}

// DisableColor disables color in output
func DisableColor() {
	pterm.DisableColor()
}

// ZarfCommand prints a zarf terminal command.
func ZarfCommand(format string, a ...any) {
	Command("zarf "+format, a...)
}

// Command prints a zarf terminal command.
func Command(format string, a ...any) {
	style := pterm.NewStyle(pterm.FgWhite, pterm.BgBlack)
	style.Printfln("$ "+format, a...)
}

// Debug prints a debug message.
func Debug(payload ...any) {
	debugPrinter(2, payload...)
}

// Debugf prints a debug message with a given format.
func Debugf(format string, a ...any) {
	message := fmt.Sprintf(format, a...)
	debugPrinter(2, message)
}

// ErrorWebf prints an error message and returns a web response.
func ErrorWebf(err any, w http.ResponseWriter, format string, a ...any) {
	debugPrinter(2, err)
	message := fmt.Sprintf(format, a...)
	Warn(message)
	http.Error(w, message, http.StatusInternalServerError)
}

// Warn prints a warning message.
func Warn(message string) {
	Warnf("%s", message)
}

// Warnf prints a warning message with a given format.
func Warnf(format string, a ...any) {
	message := Paragraphn(TermWidth-10, format, a...)
	pterm.Println()
	pterm.Warning.Println(message)
}

// WarnErr prints an error message as a warning.
func WarnErr(err any, message string) {
	debugPrinter(2, err)
	Warnf(message)
}

// WarnErrf prints an error message as a warning with a given format.
func WarnErrf(err any, format string, a ...any) {
	debugPrinter(2, err)
	Warnf(format, a...)
}

// Fatal prints a fatal error message and exits with a 1.
func Fatal(err any, message string) {
	debugPrinter(2, err)
	errorPrinter(2).Println(message)
	debugPrinter(2, string(debug.Stack()))
	os.Exit(1)
}

// Fatalf prints a fatal error message and exits with a 1 with a given format.
func Fatalf(err any, format string, a ...any) {
	message := Paragraph(format, a...)
	Fatal(err, message)
}

// Info prints an info message.
func Info(message string) {
	Infof("%s", message)
}

// Infof prints an info message with a given format.
func Infof(format string, a ...any) {
	if logLevel > 0 {
		message := Paragraph(format, a...)
		pterm.Info.Println(message)
	}
}

// Success prints a success message.
func Success(message string) {
	Successf("%s", message)
}

// Successf prints a success message with a given format.
func Successf(format string, a ...any) {
	message := Paragraph(format, a...)
	pterm.Success.Println(message)
}

// Question prints a user prompt description message.
func Question(text string) {
	Questionf("%s", text)
}

// Questionf prints a user prompt description message with a given format.
func Questionf(format string, a ...any) {
	message := Paragraph(format, a...)
	pterm.Println()
	pterm.FgLightGreen.Println(message)
}

// Note prints a note message.
func Note(text string) {
	Notef("%s", text)
}

// Notef prints a note message  with a given format.
func Notef(format string, a ...any) {
	message := Paragraphn(TermWidth-7, format, a...)
	notePrefix := pterm.PrefixPrinter{
		MessageStyle: &pterm.ThemeDefault.InfoMessageStyle,
		Prefix: pterm.Prefix{
			Style: &pterm.ThemeDefault.InfoPrefixStyle,
			Text:  "NOTE",
		},
	}
	pterm.Println()
	notePrefix.Println(message)
}

// Title prints a title and an optional help description for that section
func Title(title string, help string) {
	titleFormatted := pterm.FgBlack.Sprint(pterm.BgWhite.Sprintf(" %s ", title))
	helpFormatted := pterm.FgGray.Sprint(help)
	pterm.Printfln("%s  %s", titleFormatted, helpFormatted)
}

// HeaderInfof prints a large header with a formatted message.
func HeaderInfof(format string, a ...any) {
	pterm.Println()
	message := helpers.Truncate(fmt.Sprintf(format, a...), TermWidth, false)
	// Ensure the text is consistent for the header width
	padding := TermWidth - len(message)
	pterm.DefaultHeader.
		WithBackgroundStyle(pterm.NewStyle(pterm.BgDarkGray)).
		WithTextStyle(pterm.NewStyle(pterm.FgLightWhite)).
		WithMargin(2).
		Printfln(message + strings.Repeat(" ", padding))
}

// HorizontalRule prints a white horizontal rule to separate the terminal
func HorizontalRule() {
	pterm.Println()
	pterm.Println(RuleLine)
}

// JSONValue prints any value as JSON.
func JSONValue(value any) string {
	bytes, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		debugPrinter(2, fmt.Sprintf("ERROR marshalling json: %s", err.Error()))
	}
	return string(bytes)
}

// Paragraph formats text into a paragraph matching the TermWidth
func Paragraph(format string, a ...any) string {
	return Paragraphn(TermWidth, format, a...)
}

// Paragraphn formats text into an n column paragraph
func Paragraphn(n int, format string, a ...any) string {
	return pterm.DefaultParagraph.WithMaxWidth(n).Sprintf(format, a...)
}

// PrintDiff prints the differences between a and b with a as original and b as new
func PrintDiff(textA, textB string) {
	dmp := diffmatchpatch.New()

	diffs := dmp.DiffMain(textA, textB, true)

	diffs = dmp.DiffCleanupSemantic(diffs)

	pterm.Println(dmp.DiffPrettyText(diffs))
}

// Table prints a padded table containing the specified header and data
func Table(header []string, data [][]string) {
	pterm.Println()

	// To avoid side effects make copies of the header and data before adding padding
	headerCopy := make([]string, len(header))
	copy(headerCopy, header)
	dataCopy := make([][]string, len(data))
	copy(dataCopy, data)
	if len(headerCopy) > 0 {
		headerCopy[0] = fmt.Sprintf("     %s", headerCopy[0])
	}

	table := pterm.TableData{
		headerCopy,
	}

	for _, row := range dataCopy {
		if len(row) > 0 {
			row[0] = fmt.Sprintf("     %s", row[0])
		}
		table = append(table, pterm.TableData{row}...)
	}

	pterm.DefaultTable.WithHasHeader().WithData(table).Render()
}

// ColorWrap changes a string to an ansi color code and appends the default color to the end
// preventing future characters from taking on the given color
// returns string as normal if color is disabled
func ColorWrap(str string, attr color.Attribute) string {
	if config.NoColor {
		return str
	}
	return fmt.Sprintf("\x1b[%dm%s\x1b[0m", attr, str)
}

func debugPrinter(offset int, a ...any) {
	printer := pterm.Debug.WithShowLineNumber(logLevel > 2).WithLineNumberOffset(offset)
	now := time.Now().Format(time.RFC3339)
	// prepend to a
	a = append([]any{now, " - "}, a...)

	printer.Println(a...)

	// Always write to the log file
	if logFile != nil {
		pterm.Debug.
			WithShowLineNumber(true).
			WithLineNumberOffset(offset).
			WithDebugger(false).
			WithWriter(logFile).
			Println(a...)
	}
}

func errorPrinter(offset int) *pterm.PrefixPrinter {
	return pterm.Error.WithShowLineNumber(logLevel > 2).WithLineNumberOffset(offset)
}
