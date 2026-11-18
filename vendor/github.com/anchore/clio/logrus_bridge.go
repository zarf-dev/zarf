package clio

import (
	"fmt"
	"io"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"unicode"

	upstreamLogrus "github.com/sirupsen/logrus"

	"github.com/anchore/go-logger"
)

const (
	bridgeKey = "from"
	bridgeVal = "logrus"
)

// stdLogrusBridge routes the process-global logrus logger into the application logger. Libraries such as containerd
// and the containers/podman libraries log through logrus.StandardLogger(), which nothing configures, so without this
// their output prints raw to stderr underneath the application's logger or UI.
var stdLogrusBridge logrusBridge

type logrusBridge struct {
	// mu keeps the target and level of concurrent attach calls from interleaving. Fire only reads target.
	mu        sync.Mutex
	installed bool
	target    atomic.Pointer[logger.Logger]
}

// attach takes over src (once) and forwards its entries to dst. Repeated calls retarget the bridge and reset the
// level. src's own output is discarded, so this must not be used on a logger the application writes through directly.
func (b *logrusBridge) attach(src *upstreamLogrus.Logger, dst logger.Logger, level logger.Level) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.target.Store(&dst)
	if !b.installed {
		b.installed = true
		// add the hook before discarding output so no entries are lost in between
		src.AddHook(b)
		// formatting entries that are written to io.Discard is wasted work
		src.SetFormatter(nopFormatter{})
		src.SetOutput(io.Discard)
	}
	src.SetLevel(bridgeLevel(level))
}

// bridgeLevel picks the most verbose logrus level whose entries would still be visible at the given level, keeping in
// mind that Fire demotes info entries to debug. Levels are matched exactly, like the go-logger logrus adapter does, so
// anything it would treat as disabled is disabled here too.
func bridgeLevel(level logger.Level) upstreamLogrus.Level {
	switch level {
	case logger.TraceLevel:
		return upstreamLogrus.TraceLevel
	case logger.DebugLevel:
		return upstreamLogrus.DebugLevel
	case logger.InfoLevel, logger.WarnLevel:
		return upstreamLogrus.WarnLevel
	case logger.ErrorLevel:
		return upstreamLogrus.ErrorLevel
	}
	// disabled (e.g. -q). Note this also hides a library error logged just before it exits the process, such as
	// go.podman.io/storage's unshare setup, so the process can exit non-zero without a message.
	return upstreamLogrus.PanicLevel
}

func (b *logrusBridge) Levels() []upstreamLogrus.Level {
	return upstreamLogrus.AllLevels
}

func (b *logrusBridge) Fire(entry *upstreamLogrus.Entry) error {
	dst := b.target.Load()
	// guard against recursion when the target is itself backed by the bridged logger: our own forwarded entry would
	// come back through this hook carrying the bridge field
	if dst == nil || entry.Data[bridgeKey] == bridgeVal {
		return nil
	}

	fields := make([]any, 0, 2*len(entry.Data)+2)
	for k, v := range entry.Data {
		if dropBridgedField(k) {
			continue
		}
		fields = append(fields, sanitize(k), bridgeValue(v))
	}
	// set last so it wins over any existing "from" field
	fields = append(fields, bridgeKey, bridgeVal)

	l := (*dst).Nested(fields...)
	msg := sanitize(entry.Message)
	switch entry.Level {
	case upstreamLogrus.PanicLevel, upstreamLogrus.FatalLevel, upstreamLogrus.ErrorLevel:
		l.Error(msg)
	case upstreamLogrus.WarnLevel:
		l.Warn(msg)
	case upstreamLogrus.TraceLevel:
		l.Trace(msg)
	default:
		// libraries tend to be chatty at info, which is debug-level detail from the application's point of view
		l.Debug(msg)
	}
	return nil
}

// dropBridgedField filters fields that can carry secrets the redactor has no way of knowing about, such as
// containerd's per-request HTTP headers (e.g. a registry session cookie in "response.header.set-cookie") and raw
// token endpoint error bodies. This is best effort: a library that formats a whole response into the message itself
// (go.podman.io/image does this at debug) can't be filtered here.
func dropBridgedField(key string) bool {
	key = strings.ToLower(key)
	return key == "body" || strings.HasPrefix(key, "request.header.") || strings.HasPrefix(key, "response.header.")
}

// bridgeValue keeps scalars and flattens errors and stringers, and replaces anything else with its type name. Libraries
// attach whole structs that would otherwise be printed in full, e.g. containerd's "resp" field holds an *http.Response,
// Set-Cookie headers included.
func bridgeValue(v any) any {
	switch vv := v.(type) {
	case nil:
		return nil
	case *url.URL:
		// String() would include any password in the userinfo
		return sanitize(vv.Redacted())
	case error, fmt.Stringer:
		// fmt rather than calling the method directly so a nil receiver can't panic the logging call
		return sanitize(fmt.Sprint(vv))
	}
	switch reflect.ValueOf(v).Kind() {
	case reflect.String:
		return sanitize(reflect.ValueOf(v).String())
	case reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return v
	}
	return fmt.Sprintf("<%T>", v)
}

// sanitize replaces control characters (except tabs) with spaces, so text that originates from a remote server can't
// forge log lines or emit terminal escape sequences.
func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		if r != '\t' && unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
}

type nopFormatter struct{}

func (nopFormatter) Format(*upstreamLogrus.Entry) ([]byte, error) {
	return nil, nil
}
