package httpx

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/henvic/httpretty"

	"github.com/gpustack/gguf-parser-go/util/bytex"
)

// DefaultClient is similar to the default http.Client used by the package.
//
// It is used for requests pooling.
var DefaultClient = &http.Client{
	Transport: DefaultTransport,
}

// DefaultInsecureClient is the default http.Client used by the package,
// with TLS insecure skip verify.
//
// It is used for requests pooling.
var DefaultInsecureClient = &http.Client{
	Transport: DefaultInsecureTransport,
}

// Client returns a new http.Client with the given options,
// the result http.Client is used for fast-consuming requests.
//
// If you want a requests pool management, use DefaultClient instead.
func Client(opts ...*ClientOption) *http.Client {
	var o *ClientOption
	if len(opts) > 0 {
		o = opts[0]
	} else {
		o = ClientOptions()
	}

	root := DefaultTransport
	if o.transport != nil {
		root = o.transport
	}

	if o.debug {
		pretty := &httpretty.Logger{
			Time:            true,
			TLS:             true,
			RequestHeader:   true,
			RequestBody:     true,
			MaxRequestBody:  1024,
			ResponseHeader:  true,
			ResponseBody:    true,
			MaxResponseBody: 1024,
			Formatters:      []httpretty.Formatter{&JSONFormatter{}},
		}
		root = pretty.RoundTripper(root)
	}

	rtc := RoundTripperChain{
		Next: root,
	}
	for i := range o.roundTrippers {
		rtc = RoundTripperChain{
			Do:   o.roundTrippers[i],
			Next: rtc,
		}
	}

	var rt http.RoundTripper = rtc
	if o.retryIf != nil {
		rt = RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			for i := 0; ; i++ {
				resp, err := rtc.RoundTrip(req)
				if !o.retryIf(resp, err) {
					return resp, err
				}
				w, ok := o.retryBackoff(i+1, resp)
				if !ok {
					return resp, err
				}
				wt := time.NewTimer(w)
				select {
				case <-req.Context().Done():
					wt.Stop()
					return resp, req.Context().Err()
				case <-wt.C:
				}
			}
		})
	}

	return &http.Client{
		Transport: rt,
		Timeout:   o.timeout,
	}
}

// NewGetRequestWithContext returns a new http.MethodGet request,
// which is saving your life from http.NewRequestWithContext.
func NewGetRequestWithContext(ctx context.Context, uri string) (*http.Request, error) {
	return http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
}

// NewGetRequest returns a new http.MethodGet request,
// which is saving your life from http.NewRequest.
func NewGetRequest(uri string) (*http.Request, error) {
	return http.NewRequest(http.MethodGet, uri, nil)
}

// NewHeadRequestWithContext returns a new http.MethodHead request,
// which is saving your life from http.NewRequestWithContext.
func NewHeadRequestWithContext(ctx context.Context, uri string) (*http.Request, error) {
	return http.NewRequestWithContext(ctx, http.MethodHead, uri, nil)
}

// NewHeadRequest returns a new http.MethodHead request,
// which is saving your life from http.NewRequest.
func NewHeadRequest(uri string) (*http.Request, error) {
	return http.NewRequest(http.MethodHead, uri, nil)
}

// NewPostRequestWithContext returns a new http.MethodPost request with the given context,
// which is saving your life from http.NewRequestWithContext.
func NewPostRequestWithContext(ctx context.Context, uri string, body io.Reader) (*http.Request, error) {
	return http.NewRequestWithContext(ctx, http.MethodPost, uri, body)
}

// NewPostRequest returns a new http.MethodPost request,
// which is saving your life from http.NewRequest.
func NewPostRequest(uri string, body io.Reader) (*http.Request, error) {
	return http.NewRequest(http.MethodPost, uri, body)
}

// NewPutRequestWithContext returns a new http.MethodPut request with the given context,
// which is saving your life from http.NewRequestWithContext.
func NewPutRequestWithContext(ctx context.Context, uri string, body io.Reader) (*http.Request, error) {
	return http.NewRequestWithContext(ctx, http.MethodPut, uri, body)
}

// NewPutRequest returns a new http.MethodPut request,
// which is saving your life from http.NewRequest.
func NewPutRequest(uri string, body io.Reader) (*http.Request, error) {
	return http.NewRequest(http.MethodPut, uri, body)
}

// NewPatchRequestWithContext returns a new http.MethodPatch request with the given context,
// which is saving your life from http.NewRequestWithContext.
func NewPatchRequestWithContext(ctx context.Context, uri string, body io.Reader) (*http.Request, error) {
	return http.NewRequestWithContext(ctx, http.MethodPatch, uri, body)
}

// NewPatchRequest returns a new http.MethodPatch request,
// which is saving your life from http.NewRequest.
func NewPatchRequest(uri string, body io.Reader) (*http.Request, error) {
	return http.NewRequest(http.MethodPatch, uri, body)
}

// NewDeleteRequestWithContext returns a new http.MethodDelete request with the given context,
// which is saving your life from http.NewRequestWithContext.
func NewDeleteRequestWithContext(ctx context.Context, uri string) (*http.Request, error) {
	return http.NewRequestWithContext(ctx, http.MethodDelete, uri, nil)
}

// NewDeleteRequest returns a new http.MethodDelete request,
// which is saving your life from http.NewRequest.
func NewDeleteRequest(uri string) (*http.Request, error) {
	return http.NewRequest(http.MethodDelete, uri, nil)
}

// NewConnectRequestWithContext returns a new http.MethodConnect request with the given context,
// which is saving your life from http.NewRequestWithContext.
func NewConnectRequestWithContext(ctx context.Context, uri string) (*http.Request, error) {
	return http.NewRequestWithContext(ctx, http.MethodConnect, uri, nil)
}

// NewConnectRequest returns a new http.MethodConnect request,
// which is saving your life from http.NewRequest.
func NewConnectRequest(uri string) (*http.Request, error) {
	return http.NewRequest(http.MethodConnect, uri, nil)
}

// NewOptionsRequestWithContext returns a new http.MethodOptions request with the given context,
// which is saving your life from http.NewRequestWithContext.
func NewOptionsRequestWithContext(ctx context.Context, uri string) (*http.Request, error) {
	return http.NewRequestWithContext(ctx, http.MethodOptions, uri, nil)
}

// NewOptionsRequest returns a new http.MethodOptions request,
// which is saving your life from http.NewRequest.
func NewOptionsRequest(uri string) (*http.Request, error) {
	return http.NewRequest(http.MethodOptions, uri, nil)
}

// NewTraceRequestWithContext returns a new http.MethodTrace request with the given context,
// which is saving your life from http.NewRequestWithContext.
func NewTraceRequestWithContext(ctx context.Context, uri string) (*http.Request, error) {
	return http.NewRequestWithContext(ctx, http.MethodTrace, uri, nil)
}

// NewTraceRequest returns a new http.MethodTrace request,
// which is saving your life from http.NewRequest.
func NewTraceRequest(uri string) (*http.Request, error) {
	return http.NewRequest(http.MethodTrace, uri, nil)
}

// Error is similar to http.Error,
// but it can get the error message by the given code.
func Error(rw http.ResponseWriter, code int) {
	http.Error(rw, http.StatusText(code), code)
}

// Close closes the http response body without error.
func Close(resp *http.Response) {
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
}

// BodyBytes returns the body of the http response as a byte slice.
func BodyBytes(resp *http.Response) []byte {
	buf := bytex.GetBytes()
	defer bytex.Put(buf)

	w := bytex.GetBuffer()
	_, _ = io.CopyBuffer(w, resp.Body, buf)
	return w.Bytes()
}

// BodyString returns the body of the http response as a string.
func BodyString(resp *http.Response) string {
	return string(BodyBytes(resp))
}

// Do is a helper function to execute the given http request with the given http client,
// and execute the given function with the http response.
//
// It is useful to avoid forgetting to close the http response body.
//
// Do will return the error if failed to execute the http request or the given function.
func Do(cli *http.Client, req *http.Request, respFunc func(*http.Response) error) error {
	resp, err := cli.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer Close(resp)
	if respFunc == nil {
		return nil
	}
	return respFunc(resp)
}
