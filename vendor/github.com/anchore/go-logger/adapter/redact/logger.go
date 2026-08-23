package redact

import (
	"fmt"
	"io"

	iface "github.com/anchore/go-logger"
)

var _ iface.Logger = (*redactingLogger)(nil)
var _ iface.Controller = (*redactingLogger)(nil)

type redactingLogger struct {
	log      iface.MessageLogger
	redactor Redactor
}

func New(log iface.MessageLogger, redactor Redactor) iface.Logger {
	if r, ok := log.(*redactingLogger); ok {
		// this is already a redacting logger, so just return it, but attach it to all discovered existing stores
		r.redactor = newRedactorCollection(r.redactor, redactor)
		return r
	}
	return &redactingLogger{
		log:      log,
		redactor: redactor,
	}
}

func (r *redactingLogger) SetOutput(writer io.Writer) {
	if c, ok := r.log.(iface.Controller); ok {
		c.SetOutput(writer)
	}
}

func (r *redactingLogger) GetOutput() io.Writer {
	if c, ok := r.log.(iface.Controller); ok {
		return c.GetOutput()
	}
	return nil
}

func (r *redactingLogger) Errorf(format string, args ...any) {
	r.log.Errorf(r.redactString(format), r.redactFields(args)...)
}

func (r *redactingLogger) Error(args ...any) {
	r.log.Error(r.redactFields(args)...)
}

func (r *redactingLogger) Warnf(format string, args ...any) {
	r.log.Warnf(r.redactString(format), r.redactFields(args)...)
}

func (r *redactingLogger) Warn(args ...any) {
	r.log.Warn(r.redactFields(args)...)
}

func (r *redactingLogger) Infof(format string, args ...any) {
	r.log.Infof(r.redactString(format), r.redactFields(args)...)
}

func (r *redactingLogger) Info(args ...any) {
	r.log.Info(r.redactFields(args)...)
}

func (r *redactingLogger) Debugf(format string, args ...any) {
	r.log.Debugf(r.redactString(format), r.redactFields(args)...)
}

func (r *redactingLogger) Debug(args ...any) {
	r.log.Debug(r.redactFields(args)...)
}

func (r *redactingLogger) Tracef(format string, args ...any) {
	r.log.Tracef(r.redactString(format), r.redactFields(args)...)
}

func (r *redactingLogger) Trace(args ...any) {
	r.log.Trace(r.redactFields(args)...)
}

func (r *redactingLogger) WithFields(fields ...any) iface.MessageLogger {
	if l, ok := r.log.(iface.FieldLogger); ok {
		return New(l.WithFields(r.redactFields(fields)...), r.redactor)
	}
	return r
}

func (r *redactingLogger) Nested(fields ...any) iface.Logger {
	if l, ok := r.log.(iface.NestedLogger); ok {
		return New(l.Nested(r.redactFields(fields)...), r.redactor)
	}
	return r
}

func (r *redactingLogger) redactFields(fields []any) []any {
	for i, v := range fields {
		switch vv := v.(type) {
		case string:
			fields[i] = r.redactString(vv)
		case int, int32, int64, int16, int8, float32, float64:
			// don't coerce non-string primitives to different types
			fields[i] = vv
		case iface.Fields:
			for kkk, vvv := range vv {
				delete(vv, kkk) // this key may have data that should be redacted
				redactedKey := r.redactString(kkk)

				switch vvvv := vvv.(type) {
				case string:
					vv[redactedKey] = r.redactString(vvvv)
				case int, int32, int64, int16, int8, float32, float64:
					// don't coerce non-string primitives to different types (but still redact the key)
					vv[redactedKey] = vvvv
				default:
					vv[redactedKey] = r.redactString(fmt.Sprintf("%+v", vvvv))
				}
			}
			fields[i] = vv
		default:
			// coerce to a string and redact
			fields[i] = r.redactString(fmt.Sprintf("%+v", vv))
		}
	}
	return fields
}

func (r *redactingLogger) redactString(s string) string {
	return r.redactor.RedactString(s)
}
