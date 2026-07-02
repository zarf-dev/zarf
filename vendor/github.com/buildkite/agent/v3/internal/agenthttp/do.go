package agenthttp

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptrace"
	"net/http/httputil"
	"net/textproto"
	"strconv"
	"strings"
	"time"

	"github.com/buildkite/agent/v3/logger"
)

// Do wraps the http.Client's Do method with debug logging and tracing options.
func Do(l logger.Logger, client *http.Client, req *http.Request, opts ...DoOption) (*http.Response, error) {
	var cfg doConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.debugHTTP {
		// If the request is a multi-part form, then it's probably a
		// file upload, in which case we don't want to spewing out the
		// file contents into the debug log (especially if it's been
		// gzipped)
		dumpBody := !strings.Contains(req.Header.Get("Content-Type"), "multipart/form-data")
		requestDump, err := httputil.DumpRequestOut(req, dumpBody)
		if err != nil {
			l.Debug("ERR: %s\n%s", err, string(requestDump))
		} else {
			l.Debug("%s", string(requestDump))
		}
	}

	tracer := &tracer{Logger: l}
	if cfg.traceHTTP {
		// Inject a custom http tracer
		req = traceHTTPRequest(req, tracer)
		tracer.Start()
	}

	ts := time.Now()

	l.Debug("%s %s", req.Method, req.URL)

	resp, err := client.Do(req)
	if err != nil {
		if cfg.traceHTTP {
			tracer.EmitTraceToLog(logger.ERROR)
		}
		return nil, err
	}

	l.WithFields(
		logger.StringField("proto", resp.Proto),
		logger.IntField("status", resp.StatusCode),
		logger.DurationField("Δ", time.Since(ts)),
	).Debug("↳ %s %s", req.Method, req.URL)

	if cfg.debugHTTP {
		responseDump, err := httputil.DumpResponse(resp, true)
		if err != nil {
			l.Debug("\nERR: %s\n%s", err, string(responseDump))
		} else {
			l.Debug("\n%s", string(responseDump))
		}
	}
	if cfg.traceHTTP {
		tracer.EmitTraceToLog(logger.DEBUG)
	}

	return resp, err
}

type DoOption = func(*doConfig)

type doConfig struct {
	debugHTTP bool
	traceHTTP bool
}

func WithDebugHTTP(d bool) DoOption { return func(c *doConfig) { c.debugHTTP = d } }
func WithTraceHTTP(t bool) DoOption { return func(c *doConfig) { c.traceHTTP = t } }

type tracer struct {
	startTime time.Time
	logger.Logger
}

func (t *tracer) Start() {
	t.startTime = time.Now()
}

func (t *tracer) LogTiming(event string) {
	t.Logger = t.WithFields(logger.DurationField(event, time.Since(t.startTime)))
}

func (t *tracer) LogField(key, value string) {
	t.Logger = t.WithFields(logger.StringField(key, value))
}

func (t *tracer) LogDuration(event string, d time.Duration) {
	t.Logger = t.WithFields(logger.DurationField(event, d))
}

// Currently logger.Logger doesn't give us a way to set the level we want to emit logs at dynamically
func (t *tracer) EmitTraceToLog(level logger.Level) {
	msg := "HTTP Timing Trace"
	switch level {
	case logger.DEBUG:
		t.Debug(msg)
	case logger.INFO:
		t.Info(msg)
	case logger.WARN:
		t.Warn(msg)
	case logger.ERROR:
		t.Error(msg)
	}
}

func traceHTTPRequest(req *http.Request, t *tracer) *http.Request {
	trace := &httptrace.ClientTrace{
		GetConn: func(hostPort string) {
			t.LogField("hostPort", hostPort)
			t.LogTiming("getConn")
		},
		GotConn: func(info httptrace.GotConnInfo) {
			t.LogTiming("gotConn")
			t.LogField("reused", strconv.FormatBool(info.Reused))
			t.LogField("idle", strconv.FormatBool(info.WasIdle))
			t.LogDuration("idleTime", info.IdleTime)
			t.LogField("localAddr", info.Conn.LocalAddr().String())
		},
		PutIdleConn: func(err error) {
			t.LogTiming("putIdleConn")
			if err != nil {
				t.LogField("putIdleConnectionError", err.Error())
			}
		},
		GotFirstResponseByte: func() {
			t.LogTiming("gotFirstResponseByte")
		},
		Got1xxResponse: func(code int, header textproto.MIMEHeader) error {
			t.LogTiming("got1xxResponse")
			return nil
		},
		DNSStart: func(_ httptrace.DNSStartInfo) {
			t.LogTiming("dnsStart")
		},
		DNSDone: func(_ httptrace.DNSDoneInfo) {
			t.LogTiming("dnsDone")
		},
		ConnectStart: func(network, addr string) {
			t.LogTiming(fmt.Sprintf("connectStart.%s.%s", network, addr))
		},
		ConnectDone: func(network, addr string, _ error) {
			t.LogTiming(fmt.Sprintf("connectDone.%s.%s", network, addr))
		},
		TLSHandshakeStart: func() {
			t.LogTiming("tlsHandshakeStart")
		},
		TLSHandshakeDone: func(_ tls.ConnectionState, _ error) {
			t.LogTiming("tlsHandshakeDone")
		},
		WroteHeaders: func() {
			t.LogTiming("wroteHeaders")
		},
		WroteRequest: func(_ httptrace.WroteRequestInfo) {
			t.LogTiming("wroteRequest")
		},
	}

	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

	t.LogField("uri", req.URL.String())
	t.LogField("method", req.Method)
	return req
}
