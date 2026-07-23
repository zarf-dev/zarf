package discard

import (
	"io"

	iface "github.com/anchore/go-logger"
)

var _ iface.Logger = (*logger)(nil)
var _ iface.Controller = (*logger)(nil)

type logger struct {
}

func New() iface.Logger {
	return &logger{}
}

func (l *logger) Tracef(_ string, _ ...any) {
}

func (l *logger) Debugf(_ string, _ ...any) {}

func (l *logger) Infof(_ string, _ ...any) {}

func (l *logger) Warnf(_ string, _ ...any) {}

func (l *logger) Errorf(_ string, _ ...any) {}

func (l *logger) Trace(_ ...any) {}

func (l *logger) Debug(_ ...any) {}

func (l *logger) Info(_ ...any) {}

func (l *logger) Warn(_ ...any) {}

func (l *logger) Error(_ ...any) {}

func (l *logger) WithFields(_ ...any) iface.MessageLogger {
	return l
}

func (l *logger) Nested(_ ...any) iface.Logger { return l }

func (l *logger) SetOutput(_ io.Writer) {}

func (l *logger) GetOutput() io.Writer { return nil }
