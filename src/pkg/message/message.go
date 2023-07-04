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

// LogWriter is the stream to write logs to.
var LogWriter io.Writer = os.Stderr

var logLevel = InfoLevel

// Write logs to stderr and a buffer for logFile generation.
var logFile *os.File

var useLogFile bool

// DebugWriter represents a writer interface that writes to message.Debug
type DebugWriter struct{}

// ErrorWithMessage is a simple struct of an error and a string primarily intended for use with channels
type ErrorWithMessage struct {
	Error   error
	Message string
}

func (d *DebugWriter) Write(raw []byte) (int, error) {
	Debug(string(raw))
	return len(raw), nil
}

func init() {
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

	pterm.SetDefaultOutput(os.Stderr)
}

// UseLogFile writes output to stderr and a logFile.
func UseLogFile() {
	// Prepend the log filename with a timestamp.
	ts := time.Now().Format("2006-01-02-15-04-05")

	var err error
	if logFile != nil {
		// Use the existing log file if logFile is set
		LogWriter = io.MultiWriter(os.Stderr, logFile)
		pterm.SetDefaultOutput(LogWriter)
	} else {
		// Try to create a temp log file if one hasn't been made already
		if logFile, err = os.CreateTemp("", fmt.Sprintf("zarf-%s-*.log", ts)); err != nil {
			WarnErr(err, "Error saving a log file to a temporary directory")
		} else {
			useLogFile = true
			LogWriter = io.MultiWriter(os.Stderr, logFile)
			pterm.SetDefaultOutput(LogWriter)
			message := fmt.Sprintf("Saving log file to %s", logFile.Name())
			Note(message)
		}
	}
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
	Debug(message)
}

// ErrorWebf prints an error message and returns a web response.
func ErrorWebf(err any, w http.ResponseWriter, format string, a ...any) {
	Debug(err)
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
	Debug(err)
	Warnf(message)
}

// WarnErrorf prints an error message as a warning with a given format.
func WarnErrorf(err any, format string, a ...any) {
	Debug(err)
	Warnf(format, a...)
}

// Fatal prints a fatal error message and exits with a 1.
func Fatal(err any, message string) {
	Debug(err)
	errorPrinter(2).Println(message)
	Debug(string(debug.Stack()))
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
	pterm.Println()
	message := Paragraph(format, a...)
	pterm.FgLightGreen.Println(message)
}

// Note prints a note message.
func Note(text string) {
	Notef("%s", text)
}

// Notef prints a note message  with a given format.
func Notef(format string, a ...any) {
	pterm.Println()
	message := Paragraphn(TermWidth-7, format, a...)
	notePrefix := pterm.PrefixPrinter{
		MessageStyle: &pterm.ThemeDefault.InfoMessageStyle,
		Prefix: pterm.Prefix{
			Style: &pterm.ThemeDefault.InfoPrefixStyle,
			Text:  "NOTE",
		},
	}
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
	message := Truncate(fmt.Sprintf(format, a...), TermWidth, false)
	// Ensure the text is consistent for the header width
	padding := TermWidth - len(message)
	pterm.Println()
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
		Debug(err, "ERROR marshalling json")
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

// Truncate truncates provided text to the requested length
func Truncate(text string, length int, invert bool) string {
	// Remove newlines and replace with semicolons
	textEscaped := strings.ReplaceAll(text, "\n", "; ")
	// Truncate the text if it is longer than length so it isn't too long.
	if len(textEscaped) > length {
		if invert {
			start := len(textEscaped) - length + 3
			textEscaped = "..." + textEscaped[start:]
		} else {
			end := length - 3
			textEscaped = textEscaped[:end] + "..."
		}
	}
	return textEscaped
}

func debugPrinter(offset int, a ...any) {
	printer := pterm.Debug.WithShowLineNumber(logLevel > 2).WithLineNumberOffset(offset)
	now := time.Now().Format(time.RFC3339)
	// prepend to a
	a = append([]any{now, " - "}, a...)

	printer.Println(a...)

	// Always write to the log file
	if useLogFile {
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
