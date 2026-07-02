package log

import (
	"github.com/anchore/go-logger"
	"github.com/anchore/go-logger/adapter/discard"
)

var Log logger.Logger = discard.New()

func Errorf(format string, args ...any) {
	Log.Errorf(format, args...)
}

func Error(args ...any) {
	Log.Error(args...)
}

func Warn(args ...any) {
	Log.Warn(args...)
}

func Warnf(format string, args ...any) {
	Log.Warnf(format, args...)
}

func Infof(format string, args ...any) {
	Log.Infof(format, args...)
}

func Info(args ...any) {
	Log.Info(args...)
}

func Debugf(format string, args ...any) {
	Log.Debugf(format, args...)
}

func Debug(args ...any) {
	Log.Debug(args...)
}

// Tracef takes a formatted template string and template arguments for the trace logging level.
func Tracef(format string, args ...any) {
	Log.Tracef(format, args...)
}

// Trace logs the given arguments at the trace logging level.
func Trace(args ...any) {
	Log.Trace(args...)
}

// WithFields returns a message logger with multiple key-value fields.
func WithFields(fields ...any) logger.MessageLogger {
	return Log.WithFields(fields...)
}

// Nested returns a new logger with hard coded key-value pairs
func Nested(fields ...any) logger.Logger {
	return Log.Nested(fields...)
}
