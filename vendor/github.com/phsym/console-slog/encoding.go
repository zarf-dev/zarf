package console

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime"
	"time"
)

type encoder struct {
	opts HandlerOptions
}

func (e encoder) NewLine(buf *buffer) {
	buf.AppendByte('\n')
}

func (e encoder) withColor(b *buffer, c ANSIMod, f func()) {
	if c == "" || e.opts.NoColor {
		f()
		return
	}
	b.AppendString(string(c))
	f()
	b.AppendString(string(ResetMod))
}

func (e encoder) writeColoredTime(w *buffer, t time.Time, format string, c ANSIMod) {
	e.withColor(w, c, func() {
		w.AppendTime(t, format)
	})
}

func (e encoder) writeColoredString(w *buffer, s string, c ANSIMod) {
	e.withColor(w, c, func() {
		w.AppendString(s)
	})
}

func (e encoder) writeColoredInt(w *buffer, i int64, c ANSIMod) {
	e.withColor(w, c, func() {
		w.AppendInt(i)
	})
}

func (e encoder) writeColoredUint(w *buffer, i uint64, c ANSIMod) {
	e.withColor(w, c, func() {
		w.AppendUint(i)
	})
}

func (e encoder) writeColoredFloat(w *buffer, i float64, c ANSIMod) {
	e.withColor(w, c, func() {
		w.AppendFloat(i)
	})
}

func (e encoder) writeColoredBool(w *buffer, b bool, c ANSIMod) {
	e.withColor(w, c, func() {
		w.AppendBool(b)
	})
}

func (e encoder) writeColoredDuration(w *buffer, d time.Duration, c ANSIMod) {
	e.withColor(w, c, func() {
		w.AppendDuration(d)
	})
}

func (e encoder) writeTimestamp(buf *buffer, tt time.Time) {
	if !tt.IsZero() {
		e.writeColoredTime(buf, tt, e.opts.TimeFormat, e.opts.Theme.Timestamp())
		buf.AppendByte(' ')
	}
}

func (e encoder) writeSource(buf *buffer, pc uintptr, cwd string) {
	frame, _ := runtime.CallersFrames([]uintptr{pc}).Next()
	if cwd != "" {
		if ff, err := filepath.Rel(cwd, frame.File); err == nil {
			frame.File = ff
		}
	}
	e.withColor(buf, e.opts.Theme.Source(), func() {
		buf.AppendString(frame.File)
		buf.AppendByte(':')
		buf.AppendInt(int64(frame.Line))
	})
	e.writeColoredString(buf, " > ", e.opts.Theme.AttrKey())
}

func (e encoder) writeMessage(buf *buffer, level slog.Level, msg string) {
	if level >= slog.LevelInfo {
		e.writeColoredString(buf, msg, e.opts.Theme.Message())
	} else {
		e.writeColoredString(buf, msg, e.opts.Theme.MessageDebug())
	}
}

func (e encoder) writeAttr(buf *buffer, a slog.Attr, group string) {
	// Elide empty Attrs.
	if a.Equal(slog.Attr{}) {
		return
	}
	value := a.Value.Resolve()
	if value.Kind() == slog.KindGroup {
		subgroup := a.Key
		if group != "" {
			subgroup = group + "." + a.Key
		}
		for _, attr := range value.Group() {
			e.writeAttr(buf, attr, subgroup)
		}
		return
	}
	buf.AppendByte(' ')
	e.withColor(buf, e.opts.Theme.AttrKey(), func() {
		if group != "" {
			buf.AppendString(group)
			buf.AppendByte('.')
		}
		buf.AppendString(a.Key)
		buf.AppendByte('=')
	})
	e.writeValue(buf, value)
}

func (e encoder) writeValue(buf *buffer, value slog.Value) {
	attrValue := e.opts.Theme.AttrValue()
	switch value.Kind() {
	case slog.KindInt64:
		e.writeColoredInt(buf, value.Int64(), attrValue)
	case slog.KindBool:
		e.writeColoredBool(buf, value.Bool(), attrValue)
	case slog.KindFloat64:
		e.writeColoredFloat(buf, value.Float64(), attrValue)
	case slog.KindTime:
		e.writeColoredTime(buf, value.Time(), e.opts.TimeFormat, attrValue)
	case slog.KindUint64:
		e.writeColoredUint(buf, value.Uint64(), attrValue)
	case slog.KindDuration:
		e.writeColoredDuration(buf, value.Duration(), attrValue)
	case slog.KindAny:
		switch v := value.Any().(type) {
		case error:
			e.writeColoredString(buf, v.Error(), e.opts.Theme.AttrValueError())
			return
		case fmt.Stringer:
			e.writeColoredString(buf, v.String(), attrValue)
			return
		}
		fallthrough
	case slog.KindString:
		fallthrough
	default:
		e.writeColoredString(buf, value.String(), attrValue)
	}
}

func (e encoder) writeLevel(buf *buffer, l slog.Level) {
	var style ANSIMod
	var str string
	var delta int
	switch {
	case l >= slog.LevelError:
		style = e.opts.Theme.LevelError()
		str = "ERR"
		delta = int(l - slog.LevelError)
	case l >= slog.LevelWarn:
		style = e.opts.Theme.LevelWarn()
		str = "WRN"
		delta = int(l - slog.LevelWarn)
	case l >= slog.LevelInfo:
		style = e.opts.Theme.LevelInfo()
		str = "INF"
		delta = int(l - slog.LevelInfo)
	case l >= slog.LevelDebug:
		style = e.opts.Theme.LevelDebug()
		str = "DBG"
		delta = int(l - slog.LevelDebug)
	default:
		style = e.opts.Theme.LevelDebug()
		str = "DBG"
		delta = int(l - slog.LevelDebug)
	}
	if delta != 0 {
		str = fmt.Sprintf("%s%+d", str, delta)
	}
	e.writeColoredString(buf, str, style)
	buf.AppendByte(' ')
}
