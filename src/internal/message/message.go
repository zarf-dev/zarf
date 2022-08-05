package message

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/pterm/pterm"
)

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
)

// NoProgress tracks whether spinner/progress bars show updates
var NoProgress bool

var logLevel = InfoLevel

func init() {
	// Help capture text cleaner
	pterm.SetDefaultOutput(os.Stderr)
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

	pterm.DefaultProgressbar.MaxWidth = 85
}

func debugPrinter(offset int) *pterm.PrefixPrinter {
	return pterm.Debug.WithShowLineNumber(logLevel > 2).WithLineNumberOffset(offset)
}

func errorPrinter(offset int) *pterm.PrefixPrinter {
	return pterm.Error.WithShowLineNumber(logLevel > 2).WithLineNumberOffset(offset)
}

func SetLogLevel(lvl LogLevel) {
	logLevel = lvl
	if logLevel >= DebugLevel {
		pterm.EnableDebugMessages()
	}
}

func GetLogLevel() LogLevel {
	return logLevel
}

func Debug(payload ...any) {
	debugPrinter(1).Println(payload...)
}

func Debugf(format string, a ...any) {
	debugPrinter(2).Printfln(format, a...)
}

func Error(err any, message string) {
	debugPrinter(1).Println(err)
	Warnf(message)
}

func Errorf(err any, format string, a ...any) {
	debugPrinter(1).Println(err)
	Warnf(format, a...)
}

func Warn(message string) {
	Warnf(message)
}

func Warnf(format string, a ...any) {
	message := paragraph(format, a...)
	pterm.Warning.Println(message)
}

func Fatal(err any, message string) {
	debugPrinter(1).Println(err)
	errorPrinter(1).Println(message)
	os.Exit(1)
}

func Fatalf(err any, format string, a ...any) {
	debugPrinter(1).Println(err)
	message := paragraph(format, a...)
	errorPrinter(1).Println(message)
	os.Exit(1)
}

func Info(message string) {
	Infof(message)
}

func Infof(format string, a ...any) {
	if logLevel > 0 {
		message := paragraph(format, a...)
		pterm.Info.Println(message)
	}
}

func Question(text string) {
	pterm.Println()
	message := paragraph(text)
	pterm.FgMagenta.Println(message)
}

func Note(text string) {
	pterm.Println()
	message := paragraph(text)
	pterm.FgYellow.Println(message)
}

func Notef(text string, a ...any) {
	pterm.Println()
	message := paragraph(text, a)
	pterm.FgYellow.Println(message)
}

func HeaderInfof(format string, a ...any) {
	message := fmt.Sprintf(format, a...)
	// Ensure the text is consistent for the header width
	padding := 85 - len(message)
	pterm.Println()
	pterm.DefaultHeader.
		WithBackgroundStyle(pterm.NewStyle(pterm.BgDarkGray)).
		WithTextStyle(pterm.NewStyle(pterm.FgLightWhite)).
		WithMargin(2).
		Printfln(message + strings.Repeat(" ", padding))
}

func JsonValue(value any) string {
	bytes, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		Debugf("ERROR marshalling json: %w", err)
	}
	return string(bytes)
}

func paragraph(format string, a ...any) string {
	return pterm.DefaultParagraph.WithMaxWidth(100).Sprintf(format, a...)
}
