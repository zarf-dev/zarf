package logrus

import (
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"

	"github.com/sirupsen/logrus"

	iface "github.com/anchore/go-logger"
)

var _ iface.Logger = (*logger)(nil)
var _ iface.Controller = (*logger)(nil)

const (
	defaultLogFilePermissions fs.FileMode = 0644
	timestampFormat                       = "2006-01-02 15:04:05"
)

// Config contains all configurable values for the Logrus entry
type Config struct {
	EnableConsole     bool
	FileLocation      string
	Level             iface.Level
	Formatter         logrus.Formatter
	CaptureCallerInfo bool
	NoLock            bool
}

func DefaultConfig() Config {
	return Config{
		EnableConsole:     true,
		FileLocation:      "",
		Level:             iface.InfoLevel,
		CaptureCallerInfo: false,
		NoLock:            false,
		Formatter:         DefaultTextFormatter(),
	}
}

func DefaultTextFormatter() logrus.Formatter {
	return &TextFormatter{
		TimestampFormat: timestampFormat,
		ForceFormatting: true,
	}
}

func DefaultJSONFormatter() logrus.Formatter {
	return &logrus.JSONFormatter{
		TimestampFormat:   timestampFormat,
		DisableTimestamp:  false,
		DisableHTMLEscape: false,
		PrettyPrint:       false,
	}
}

// logger contains all runtime values for using Logrus with the configured output target and input configuration values.
type logger struct {
	config Config
	logger *logrus.Logger
	output io.Writer
}

// Use adapts the given logger based on the provided configuration
func Use(l *logrus.Logger, cfg Config) (iface.Logger, error) {
	var output io.Writer
	switch {
	case cfg.EnableConsole && cfg.FileLocation != "":
		logFile, err := os.OpenFile(cfg.FileLocation, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, defaultLogFilePermissions)
		if err != nil {
			return nil, fmt.Errorf("unable to setup log file: %w", err)
		}
		output = io.MultiWriter(os.Stderr, logFile)
	case cfg.EnableConsole:
		output = os.Stderr
	case cfg.FileLocation != "":
		logFile, err := os.OpenFile(cfg.FileLocation, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, defaultLogFilePermissions)
		if err != nil {
			return nil, fmt.Errorf("unable to setup log file: %w", err)
		}
		output = logFile
	default:
		output = io.Discard
	}

	var level logrus.Level
	if cfg.Level == iface.DisabledLevel {
		level = logrus.PanicLevel
	} else {
		level = getLogLevel(cfg.Level)
	}

	l.SetOutput(output)
	l.SetLevel(level)
	l.SetReportCaller(cfg.CaptureCallerInfo)

	if cfg.NoLock {
		l.SetNoLock()
	}

	if cfg.Formatter != nil {
		l.SetFormatter(cfg.Formatter)
	} else {
		l.SetFormatter(DefaultTextFormatter())
	}

	return &logger{
		config: cfg,
		logger: l,
		output: output,
	}, nil
}

// New creates a new logger with the given configuration
func New(cfg Config) (iface.Logger, error) {
	return Use(logrus.New(), cfg)
}

// Tracef takes a formatted template string and template arguments for the trace logging level.
func (l *logger) Tracef(format string, args ...any) {
	l.logger.Tracef(format, args...)
}

// Debugf takes a formatted template string and template arguments for the debug logging level.
func (l *logger) Debugf(format string, args ...any) {
	l.logger.Debugf(format, args...)
}

// Infof takes a formatted template string and template arguments for the info logging level.
func (l *logger) Infof(format string, args ...any) {
	l.logger.Infof(format, args...)
}

// Warnf takes a formatted template string and template arguments for the warning logging level.
func (l *logger) Warnf(format string, args ...any) {
	l.logger.Warnf(format, args...)
}

// Errorf takes a formatted template string and template arguments for the error logging level.
func (l *logger) Errorf(format string, args ...any) {
	l.logger.Errorf(format, args...)
}

// Trace logs the given arguments at the trace logging level.
func (l *logger) Trace(args ...any) {
	l.logger.Trace(args...)
}

// Debug logs the given arguments at the debug logging level.
func (l *logger) Debug(args ...any) {
	l.logger.Debug(args...)
}

// Info logs the given arguments at the info logging level.
func (l *logger) Info(args ...any) {
	l.logger.Info(args...)
}

// Warn logs the given arguments at the warning logging level.
func (l *logger) Warn(args ...any) {
	l.logger.Warn(args...)
}

// Error logs the given arguments at the error logging level.
func (l *logger) Error(args ...any) {
	l.logger.Error(args...)
}

// WithFields returns a message entry with multiple key-value fields.
func (l *logger) WithFields(fields ...any) iface.MessageLogger {
	return l.logger.WithFields(getFields(fields...))
}

func (l *logger) Nested(fields ...any) iface.Logger {
	return &nestedLogger{entry: l.logger.WithFields(getFields(fields...))}
}

func (l *logger) SetOutput(writer io.Writer) {
	l.output = writer
	l.logger.SetOutput(writer)
}

func (l *logger) GetOutput() io.Writer {
	return l.output
}

func getFields(fields ...any) logrus.Fields {
	f := make(logrus.Fields)
	offset := 0
	for i, val := range fields {
		// there can be a fields map anywhere within the parameters
		if fieldsMap, ok := val.(iface.Fields); ok {
			maps.Copy(f, fieldsMap)
			offset++
			continue
		}

		// virtually skip any field maps found when figuring if this is a key or a value
		if (i-offset)%2 != 0 {
			f[fmt.Sprintf("%s", fields[i-1])] = val
		}
	}
	return f
}

func getLogLevel(level iface.Level) logrus.Level {
	switch level {
	case iface.ErrorLevel:
		return logrus.ErrorLevel
	case iface.WarnLevel:
		return logrus.WarnLevel
	case iface.InfoLevel:
		return logrus.InfoLevel
	case iface.DebugLevel:
		return logrus.DebugLevel
	case iface.TraceLevel:
		return logrus.TraceLevel
	}
	return logrus.PanicLevel
}
