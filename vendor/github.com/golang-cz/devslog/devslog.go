package devslog

import (
	"bytes"
	"context"
	"encoding"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type developHandler struct {
	opts Options
	goas []groupOrAttrs
	mu   sync.Mutex
	out  io.Writer
}

type Options struct {
	// You can use standard slog.HandlerOptions, that would be used in production
	*slog.HandlerOptions

	// Max number of printed elements in slice.
	MaxSlicePrintSize uint

	// If the attributes should be sorted by keys
	SortKeys bool

	// Time format for timestamp, default format is "[15:04:05]"
	TimeFormat string

	// Add blank line after each log
	NewLineAfterLog bool

	// Indent \n in strings
	StringIndentation bool

	// Set color for Debug level, default: devslog.Blue
	DebugColor Color

	// Set color for Info level, default: devslog.Green
	InfoColor Color

	// Set color for Warn level, default: devslog.Yellow
	WarnColor Color

	// Set color for Error level, default: devslog.Red
	ErrorColor Color

	// Max stack trace frames when unwrapping errors
	MaxErrorStackTrace uint

	// Use method String() for formatting value
	StringerFormatter bool

	// Disable coloring
	NoColor bool

	// Keep same color for whole source info, helpful when you want to open the line of code from terminal, but the ANSI coloring codes are in link itself
	SameSourceInfoColor bool
}

type groupOrAttrs struct {
	group string
	attrs []slog.Attr
}

func NewHandler(out io.Writer, o *Options) *developHandler {
	h := &developHandler{out: out}
	if o != nil {
		h.opts = *o

		if o.HandlerOptions != nil {
			h.opts.HandlerOptions = o.HandlerOptions
			if o.Level == nil {
				h.opts.Level = slog.LevelInfo
			} else {
				h.opts.Level = o.Level
			}
		} else {
			h.opts.HandlerOptions = &slog.HandlerOptions{
				Level: slog.LevelInfo,
			}
		}

		if o.MaxSlicePrintSize == 0 {
			h.opts.MaxSlicePrintSize = 50
		}

		if o.TimeFormat == "" {
			h.opts.TimeFormat = "[15:04:05]"
		}

		h.opts.DebugColor = ensureValidColor(o.DebugColor, Blue)
		h.opts.InfoColor = ensureValidColor(o.InfoColor, Green)
		h.opts.WarnColor = ensureValidColor(o.WarnColor, Yellow)
		h.opts.ErrorColor = ensureValidColor(o.ErrorColor, Red)

	} else {
		h.opts = Options{
			HandlerOptions:    &slog.HandlerOptions{Level: slog.LevelInfo},
			MaxSlicePrintSize: 50,
			SortKeys:          false,
			TimeFormat:        "[15:04:05]",
			DebugColor:        Blue,
			InfoColor:         Green,
			WarnColor:         Yellow,
			ErrorColor:        Red,
		}
	}

	return h
}

func ensureValidColor(c Color, defaultColor Color) Color {
	if c > 0 && int(c) < len(colors) {
		return c
	}

	return defaultColor
}

func (h *developHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return l >= h.opts.Level.Level()
}

func (h *developHandler) WithGroup(s string) slog.Handler {
	if s == "" {
		return h
	}

	return h.withGroupOrAttrs(groupOrAttrs{group: s})
}

func (h *developHandler) WithAttrs(as []slog.Attr) slog.Handler {
	if len(as) == 0 {
		return h
	}

	return h.withGroupOrAttrs(groupOrAttrs{attrs: as})
}

func (h *developHandler) withGroupOrAttrs(goa groupOrAttrs) *developHandler {
	h2 := &developHandler{
		opts: h.opts,
		goas: make([]groupOrAttrs, len(h.goas)+1),
		out:  h.out,
	}

	copy(h2.goas, h.goas)
	h2.goas[len(h2.goas)-1] = goa

	return h2
}

func (h *developHandler) Handle(ctx context.Context, r slog.Record) error {
	b := make([]byte, 0, 1024)
	b = append(b, h.faintedText([]byte(r.Time.Format(h.opts.TimeFormat)))...)
	b = append(b, ' ')
	b = h.formatSourceInfo(b, &r)
	b = h.levelMessage(b, &r)
	b = h.processAttributes(b, &r)

	h.mu.Lock()
	defer h.mu.Unlock()

	_, err := h.out.Write(b)

	return err
}

func (h *developHandler) formatSourceInfo(b []byte, r *slog.Record) []byte {
	if h.opts.AddSource {
		f, _ := runtime.CallersFrames([]uintptr{r.PC}).Next()
		s := &slog.Source{
			Function: f.Function,
			File:     f.File,
			Line:     f.Line,
		}

		if h.opts.ReplaceAttr != nil {
			attr := h.opts.ReplaceAttr([]string{}, slog.Any(slog.SourceKey, s))
			if attr.Key == "" {
				b = append(b, '\n')
				return b
			}
		}

		b = append(b, h.colorStringFainted([]byte("@@@"), fgYellow)...)
		b = append(b, ' ')

		if h.opts.SameSourceInfoColor {
			b = append(b, h.underlineText(h.colorString(append(append([]byte(s.File), ':'), []byte(strconv.Itoa(s.Line))...), fgBlue))...)
		} else {
			b = append(b, h.underlineText(h.colorStringFainted([]byte(s.File), fgCyan))...)
			b = append(b, h.faintedText([]byte(":"))...)
			b = append(b, h.colorStringFainted([]byte(strconv.Itoa(s.Line)), fgRed)...)
		}

		b = append(b, '\n')
	}

	return b
}

func (h *developHandler) levelMessage(b []byte, r *slog.Record) []byte {
	var ls string
	if h.opts.ReplaceAttr != nil {
		a := h.opts.ReplaceAttr(nil, slog.Any(slog.LevelKey, r.Level))
		ls = a.Value.String()
		if a.Key != "level" {
			r.AddAttrs(a)
		}
	} else {
		ls = r.Level.String()
	}

	var c color
	lr := r.Level
	switch {
	case lr < 0:
		c = h.getColor(h.opts.DebugColor)
	case lr < 4:
		c = h.getColor(h.opts.InfoColor)
	case lr < 8:
		c = h.getColor(h.opts.WarnColor)
	default:
		c = h.getColor(h.opts.ErrorColor)
	}

	b = append(b, h.colorStringBackgorund([]byte(" "+ls+" "), fgBlack, c.bg)...)
	b = append(b, ' ')
	b = append(b, h.colorString([]byte(r.Message), c.fg)...)
	b = append(b, '\n')

	return b
}

type visitKey struct {
	ptr uintptr
	typ reflect.Type
}

type visited map[visitKey]struct{}

func (h *developHandler) processAttributes(b []byte, r *slog.Record) []byte {
	var as attributes
	r.Attrs(func(a slog.Attr) bool {
		a.Value = a.Value.Resolve()
		as = append(as, a)
		return true
	})

	goas := h.goas
	if r.NumAttrs() == 0 {
		for len(goas) > 0 && goas[len(goas)-1].group != "" {
			goas = goas[:len(goas)-1]
		}
	}

	for i := len(goas) - 1; i >= 0; i-- {
		if goas[i].group != "" {
			ng := slog.Attr{
				Key:   goas[i].group,
				Value: slog.GroupValue(as...),
			}
			as = attributes{ng}
		} else {
			as = append(as, goas[i].attrs...)
		}
	}

	vi := make(visited)
	b = h.colorize(b, as, 0, []string{}, vi)
	if h.opts.NewLineAfterLog {
		b = append(b, '\n')
	}

	return b
}

func (h *developHandler) colorize(b []byte, as attributes, l int, group []string, vi visited) []byte {
	if h.opts.SortKeys {
		sort.Sort(as)
	}

	paddingNoColor := h.padding(as, group, nil, h.colorString)
	paddingColor := h.padding(as, group, fgMagenta, h.colorString)
	for _, a := range as {
		if h.opts.ReplaceAttr != nil {
			a = h.opts.ReplaceAttr(group, a)
		}

		key := h.colorString([]byte(a.Key), fgMagenta)
		val := []byte(a.Value.String())
		valOld := val
		vs := val
		mark := []byte(" ")

		switch a.Value.Kind() {
		case slog.KindFloat64, slog.KindInt64, slog.KindUint64:
			mark = h.colorString([]byte("#"), fgYellow)
			val = h.colorString(val, fgYellow)
		case slog.KindBool:
			mark = h.colorString([]byte("#"), fgRed)
			val = h.colorString(val, fgRed)
		case slog.KindString:
			if len(val) == 0 {
				val = h.colorStringFainted([]byte("empty"), fgWhite)
			} else if h.isURL(val) {
				mark = h.colorString([]byte("*"), fgBlue)
				val = h.underlineText(h.colorString(val, fgBlue))
			} else {
				if h.opts.StringIndentation {
					count := l*2 + (4 + (paddingNoColor))
					val = []byte(strings.ReplaceAll(string(val), "\n", "\n"+strings.Repeat(" ", count)))
				}
			}
		case slog.KindTime, slog.KindDuration:
			mark = h.colorString([]byte("@"), fgCyan)
			val = h.colorString(val, fgCyan)
		case slog.KindAny:
			av := a.Value.Any()
			if err, ok := av.(error); ok {
				mark = h.colorString([]byte("E"), fgRed)
				val = h.formatError(err, l)
				break
			}

			if t, ok := av.(*time.Time); ok {
				mark = h.colorString([]byte("@"), fgCyan)
				val = h.colorString([]byte(t.String()), fgCyan)
				break
			}

			if d, ok := av.(*time.Duration); ok {
				mark = h.colorString([]byte("@"), fgCyan)
				val = h.colorString([]byte(d.String()), fgCyan)
				break
			}

			if textMarshaller, ok := av.(encoding.TextMarshaler); ok {
				val = atb(textMarshaller)
				break
			}

			if h.opts.StringerFormatter {
				if stringer, ok := av.(fmt.Stringer); ok {
					val = []byte(stringer.String())
					break
				}
			}

			avt := reflect.TypeOf(av)
			avv := reflect.ValueOf(av)
			if avt == nil {
				mark = h.colorString([]byte("!"), fgRed)
				val = h.nilString()
				break
			}

			ut, uv, ptrs := h.reducePointerTypeValue(avt, avv)
			val = bytes.Repeat(h.colorString([]byte("*"), fgRed), ptrs)

			switch ut.Kind() {
			case reflect.Array:
				mark = h.colorString([]byte("A"), fgGreen)
				val = h.formatSlice(avt, avv, l, vi)
			case reflect.Slice:
				mark = h.colorString([]byte("S"), fgGreen)
				val = h.formatSlice(avt, avv, l, vi)
			case reflect.Map:
				mark = h.colorString([]byte("M"), fgGreen)
				val = h.formatMap(avt, avv, l, vi)
			case reflect.Struct:
				mark = h.colorString([]byte("S"), fgYellow)
				val = h.formatStruct(avt, avv, 0, vi)
			case reflect.Float32, reflect.Float64:
				mark = h.colorString([]byte("#"), fgYellow)
				vs = atb(uv.Float())
				val = append(val, h.colorString(vs, fgYellow)...)
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				mark = h.colorString([]byte("#"), fgYellow)
				vs = atb(uv.Int())
				val = append(val, h.colorString(vs, fgYellow)...)
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				mark = h.colorString([]byte("#"), fgYellow)
				vs = atb(uv.Uint())
				val = append(val, h.colorString(vs, fgYellow)...)
			case reflect.Bool:
				mark = h.colorString([]byte("#"), fgRed)
				vs = atb(uv.Bool())
				val = append(val, h.colorString(vs, fgRed)...)
			case reflect.String:
				s := uv.String()
				if len(s) == 0 {
					val = h.colorStringFainted([]byte("empty"), fgWhite)
				} else if h.isURL([]byte(s)) {
					val = h.underlineText(h.colorString(val, fgBlue))
				} else {
					val = []byte(uv.String())
				}
			default:
				mark = h.colorString([]byte("!"), fgRed)
				val = h.colorString(atb("Unknown type"), fgRed)
			}
		case slog.KindGroup:
			mark = h.colorString([]byte("G"), fgGreen)
			var ga attributes
			ga = a.Value.Group()
			group = append(group, a.Key)

			val = []byte("\n")
			val = append(val, h.colorize(nil, ga, l+1, group, vi)...)
		}

		b = append(b, bytes.Repeat([]byte(" "), l*2)...)
		b = append(b, mark...)
		b = append(b, ' ')
		b = append(b, key...)
		b = append(b, bytes.Repeat([]byte(" "), paddingColor-len(key))...)
		b = append(b, ':', ' ')
		b = append(b, val...)

		stringer := reflect.ValueOf(a.Value).MethodByName("String")
		if stringer.IsValid() && !bytes.Equal(valOld, vs) {
			s := []byte(` "`)
			s = append(s, []byte(a.Value.String())...)
			s = append(s, '"')
			b = append(b, h.colorStringFainted(s, fgWhite)...)
		}

		if a.Value.Kind() != slog.KindGroup {
			b = append(b, '\n')
		}
	}

	return b
}

func (h *developHandler) padding(a attributes, g []string, color foregroundColor, colorFunction func(b []byte, fgColor foregroundColor) []byte) int {
	var padding int
	for _, attr := range a {
		if h.opts.ReplaceAttr != nil {
			attr = h.opts.ReplaceAttr(g, attr)
		}

		colorLength := len(attr.Key)
		if color != nil {
			colorLength = len(colorFunction([]byte(attr.Key), color))
		}

		if colorLength > padding {
			padding = colorLength
		}
	}

	return padding
}

func (h *developHandler) isURL(u []byte) bool {
	_, err := url.ParseRequestURI(string(u))
	return err == nil
}

func (h *developHandler) formatError(err error, l int) (b []byte) {
	if err == nil {
		b = append(b, h.underlineText(h.colorString(h.nilString(), fgRed))...)

		return
	}

	for i := 0; err != nil; i++ {
		b = append(b, '\n')
		b = append(b, bytes.Repeat([]byte(" "), l*2+4)...)
		b = append(b, h.colorString([]byte(strconv.Itoa(i)), fgRed)...)
		b = append(b, h.colorString([]byte(": "), fgWhite)...)

		errMsg := err.Error()
		ue := errors.Unwrap(err)
		if ue != nil {
			errMsg, _ = strings.CutSuffix(errMsg, ue.Error())
			errMsg, _ = strings.CutSuffix(errMsg, ": ")
		}

		if errMsg == "" {
			errMsg = fmt.Sprintf("[%T]", err)
		}

		b = append(b, h.colorString([]byte(errMsg), fgRed)...)

		for j, fileLine := range h.getFileLineFromPC(h.extractPCFromError(err)) {
			b = append(b, '\n')
			tb := strconv.Itoa(j)
			b = append(b, bytes.Repeat([]byte(" "), l*2+6)...)
			b = append(b, bytes.Repeat([]byte(" "), len(tb))...)
			b = append(b, h.colorString([]byte(tb), fgBlue)...)
			b = append(b, []byte(": ")...)
			b = append(b, h.underlineText(h.colorString([]byte(fileLine), fgBlue))...)
		}

		err = ue
	}

	return b
}

func (h *developHandler) formatSlice(st reflect.Type, sv reflect.Value, l int, vi visited) (b []byte) {
	ts := h.buildTypeString(st.String())
	_, sv, _ = h.reducePointerTypeValue(st, sv)

	b = append(b, h.colorString([]byte(strconv.Itoa(sv.Len())), fgBlue)...)
	b = append(b, ' ')
	b = append(b, ts...)
	d := min(len(strconv.Itoa(int(h.opts.MaxSlicePrintSize))), len(strconv.Itoa(sv.Len())))

	for i := 0; i < sv.Len(); i++ {
		if i == int(h.opts.MaxSlicePrintSize) {
			b = append(b, '\n')
			b = append(b, bytes.Repeat([]byte(" "), l*2+4)...)
			b = append(b, bytes.Repeat([]byte(" "), d+2)...)
			b = append(b, h.colorString([]byte("..."), fgBlue)...)
			b = append(b, h.colorString([]byte("]"), fgGreen)...)
			break
		}

		v := sv.Index(i)
		t := v.Type()

		tb := strconv.Itoa(i)
		b = append(b, '\n')
		b = append(b, bytes.Repeat([]byte(" "), l*2+4)...)
		b = append(b, bytes.Repeat([]byte(" "), d-len(tb))...)
		b = append(b, h.colorString([]byte(tb), fgGreen)...)
		b = append(b, ':')
		b = append(b, ' ')
		b = append(b, h.elementType(t, v, l, l*2+d+2, vi)...)
	}

	return b
}

func (h *developHandler) formatMap(st reflect.Type, sv reflect.Value, l int, vi visited) (b []byte) {
	ts := h.buildTypeString(st.String())
	_, sv, _ = h.reducePointerTypeValue(st, sv)

	pc := h.mapKeyPadding(sv, &fgGreen)
	pr := h.mapKeyPadding(sv, nil)
	b = append(b, h.colorString([]byte(strconv.Itoa(sv.Len())), fgBlue)...)
	b = append(b, ' ')
	b = append(b, ts...)
	sk := h.sortMapKeys(sv)
	for _, k := range sk {
		v := sv.MapIndex(k)
		v = h.reducePointerValue(v)
		k = h.reducePointerValue(k)

		tb := h.colorString(atb(k.Interface()), fgGreen)
		b = append(b, '\n')
		b = append(b, bytes.Repeat([]byte(" "), l*2+4)...)
		b = append(b, tb...)
		b = append(b, bytes.Repeat([]byte(" "), pc-len(tb))...)
		b = append(b, ':')
		b = append(b, ' ')
		b = append(b, h.elementType(v.Type(), v, l, l*2+pr+2, vi)...)
	}

	return b
}

func (h *developHandler) formatStruct(st reflect.Type, sv reflect.Value, l int, vi visited) (b []byte) {
	b = h.buildTypeString(st.String())

	_, sv, _ = h.reducePointerTypeValue(st, sv)
	pc := h.structKeyPadding(sv, &fgGreen)
	pr := h.structKeyPadding(sv, nil)

	for i := 0; i < sv.NumField(); i++ {
		if !sv.Type().Field(i).IsExported() {
			continue
		}

		v := sv.Field(i)
		t := v.Type()

		tb := h.colorString([]byte(sv.Type().Field(i).Name), fgGreen)
		b = append(b, '\n')
		b = append(b, bytes.Repeat([]byte(" "), l*2+4)...)
		b = append(b, tb...)
		b = append(b, bytes.Repeat([]byte(" "), pc-len(tb))...)
		b = append(b, ':')
		b = append(b, ' ')
		b = append(b, h.elementType(t, v, l, l*2+pr+2, vi)...)
	}

	return b
}

var marshalTextInterface = reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()

func (h *developHandler) elementType(t reflect.Type, v reflect.Value, l int, p int, vi visited) (b []byte) {
	if t.Implements(marshalTextInterface) {
		return atb(v)
	}

	if h.opts.StringerFormatter {
		if stringer, ok := v.Interface().(fmt.Stringer); ok {
			return []byte(stringer.String())
		}
	}

	switch v.Kind() {
	case reflect.Array:
		b = h.formatSlice(t, v, l+1, vi)
	case reflect.Slice:
		b = h.formatSlice(t, v, l+1, vi)
	case reflect.Map:
		b = h.formatMap(t, v, l+1, vi)
	case reflect.Struct:
		b = h.formatStruct(t, v, l+1, vi)
	case reflect.Pointer:
		key := visitKey{
			ptr: v.Pointer(),
			typ: v.Type(),
		}

		if v.IsNil() {
			b = h.nilString()
		} else if _, ok := vi[key]; ok {
			b = atb(v)
		} else {
			vi[key] = struct{}{}
			b = h.elementType(t, v.Elem(), l, p, vi)
		}
	case reflect.Float32, reflect.Float64:
		b = h.colorString(atb(v.Float()), fgYellow)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		b = h.colorString(atb(v.Int()), fgYellow)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		b = h.colorString(atb(v.Uint()), fgYellow)
	case reflect.Bool:
		b = h.colorString(atb(v.Bool()), fgRed)
	case reflect.String:
		s := v.String()
		if len(s) == 0 {
			b = h.colorStringFainted([]byte("empty"), fgWhite)
		} else if h.isURL([]byte(s)) {
			b = h.underlineText(h.colorString([]byte(s), fgBlue))
		} else {
			if h.opts.StringIndentation {
				b = []byte(strings.ReplaceAll(string(s), "\n", "\n"+strings.Repeat(" ", l*2+p+4)))
			} else {
				b = atb(s)
			}
		}
	case reflect.Interface:
		if v.IsZero() {
			b = h.nilString()
		} else {
			v = reflect.ValueOf(v.Interface())
			b = h.elementType(v.Type(), v, l, p, vi)
		}
	default:
		b = atb("Unknown type: ")
		b = append(b, atb(v.Kind())...)
		b = h.colorString(b, fgRed)
	}

	return b
}

func (h *developHandler) buildTypeString(ts string) (b []byte) {
	t := []byte(ts)

	for len(t) > 0 {
		switch t[0] {
		case '*':
			b = append(b, h.colorString([]byte{t[0]}, fgRed)...)
		case '[', ']':
			b = append(b, h.colorString([]byte{t[0]}, fgGreen)...)
		default:
			b = append(b, h.colorString([]byte{t[0]}, fgYellow)...)
		}

		t = t[1:]
	}

	return b
}

func (h *developHandler) sortMapKeys(rv reflect.Value) []reflect.Value {
	ks := make([]reflect.Value, 0, rv.Len())
	ks = append(ks, rv.MapKeys()...)

	sort.Slice(ks, func(i, j int) bool {
		return fmt.Sprint(ks[i].Interface()) < fmt.Sprint(ks[j].Interface())
	})

	return ks
}

func (h *developHandler) mapKeyPadding(rv reflect.Value, fgColor *foregroundColor) (p int) {
	for _, k := range rv.MapKeys() {
		k = h.reducePointerValue(k)
		c := len(atb(k.Interface()))
		if fgColor != nil {
			c = len(h.colorString(atb(k.Interface()), *fgColor))
		}

		if c > p {
			p = c
		}
	}

	return p
}

func (h *developHandler) structKeyPadding(sv reflect.Value, fgColor *foregroundColor) (p int) {
	st := sv.Type()
	for i := 0; i < sv.NumField(); i++ {
		if !st.Field(i).IsExported() {
			continue
		}

		c := len(st.Field(i).Name)
		if fgColor != nil {
			c = len(h.colorString([]byte(st.Field(i).Name), *fgColor))
		}

		if c > p {
			p = c
		}
	}

	return p
}

func (h *developHandler) reducePointerValue(v reflect.Value) reflect.Value {
	for v.Kind() == reflect.Pointer {
		v = v.Elem()
	}

	return v
}

func (h *developHandler) reducePointerTypeValue(t reflect.Type, v reflect.Value) (reflect.Type, reflect.Value, int) {
	if t == nil {
		return t, v, 0
	}

	var ptr int
	for v.Kind() == reflect.Pointer {
		v = v.Elem()
		if isNilValue(v) {
			return t, v, ptr
		}

		t = v.Type()

		ptr++
	}

	return t, v, ptr
}

// Any to []byte using fmt.Sprintf
func atb(a any) []byte {
	return fmt.Appendf(nil, "%v", a)
}

func isNilValue(v reflect.Value) bool {
	nilValue := reflect.ValueOf(nil)
	return v == nilValue
}

func (h *developHandler) nilString() []byte {
	b := h.colorString([]byte("<"), fgRed)
	b = append(b, h.colorString([]byte("nil"), fgYellow)...)
	b = append(b, h.colorString([]byte(">"), fgRed)...)
	return b
}
