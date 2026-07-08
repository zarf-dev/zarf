package httpx

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type ClientOption struct {
	*TransportOption

	timeout       time.Duration
	debug         bool
	retryIf       RetryFunc
	retryBackoff  func(attemptNum int, resp *http.Response) (wait time.Duration, ok bool)
	roundTrippers []func(req *http.Request) error
}

func ClientOptions() *ClientOption {
	return &ClientOption{
		TransportOption: TransportOptions().WithoutKeepalive(),
		timeout:         30 * time.Second,
		retryIf:         DefaultRetry,
		retryBackoff:    createRetryBackoff(100*time.Millisecond, 5*time.Second, 5),
	}
}

// WithTransport sets the TransportOption.
func (o *ClientOption) WithTransport(opt *TransportOption) *ClientOption {
	if o == nil || opt == nil {
		return o
	}
	o.TransportOption = opt
	return o
}

// WithTimeout sets the request timeout.
//
// This timeout controls the sum of [network dial], [tls handshake], [request], [response header reading] and [response body reading].
//
// Use 0 to disable timeout.
func (o *ClientOption) WithTimeout(timeout time.Duration) *ClientOption {
	if o == nil || timeout < 0 {
		return o
	}
	o.timeout = timeout
	return o
}

// WithDebug sets the debug mode.
func (o *ClientOption) WithDebug() *ClientOption {
	if o == nil {
		return o
	}
	o.debug = true
	return o
}

type RetryFunc func(resp *http.Response, err error) (retry bool)

// WithRetryIf specifies the if-condition of retry operation for request,
// or stops retrying if setting with `nil`.
func (o *ClientOption) WithRetryIf(retryIf RetryFunc) *ClientOption {
	if o == nil {
		return o
	}
	o.retryIf = retryIf
	return o
}

// WithRetryBackoff specifies the retry-backoff mechanism for request.
func (o *ClientOption) WithRetryBackoff(waitMin, waitMax time.Duration, attemptMax int) *ClientOption {
	if o == nil || waitMin < 0 || waitMax < 0 || waitMax < waitMin || attemptMax <= 0 {
		return o
	}
	o.retryBackoff = createRetryBackoff(waitMin, waitMax, attemptMax)
	return o
}

// WithUserAgent sets the user agent.
func (o *ClientOption) WithUserAgent(ua string) *ClientOption {
	return o.WithRoundTripper(func(req *http.Request) error {
		req.Header.Set("User-Agent", ua)
		return nil
	})
}

// WithBearerAuth sets the bearer token.
func (o *ClientOption) WithBearerAuth(token string) *ClientOption {
	return o.WithRoundTripper(func(req *http.Request) error {
		req.Header.Set("Authorization", "Bearer "+token)
		return nil
	})
}

// WithBasicAuth sets the basic authentication.
func (o *ClientOption) WithBasicAuth(username, password string) *ClientOption {
	return o.WithRoundTripper(func(req *http.Request) error {
		req.SetBasicAuth(username, password)
		return nil
	})
}

// WithHeader sets the header.
func (o *ClientOption) WithHeader(key, value string) *ClientOption {
	return o.WithRoundTripper(func(req *http.Request) error {
		req.Header.Set(key, value)
		return nil
	})
}

// WithHeaders sets the headers.
func (o *ClientOption) WithHeaders(headers map[string]string) *ClientOption {
	return o.WithRoundTripper(func(req *http.Request) error {
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		return nil
	})
}

// WithRoundTripper sets the round tripper.
func (o *ClientOption) WithRoundTripper(rt func(req *http.Request) error) *ClientOption {
	if o == nil || rt == nil {
		return o
	}
	o.roundTrippers = append(o.roundTrippers, rt)
	return o
}

// If is a conditional option,
// which receives a boolean condition to trigger the given function or not.
func (o *ClientOption) If(condition bool, then func(*ClientOption) *ClientOption) *ClientOption {
	if condition {
		return then(o)
	}
	return o
}

// DefaultRetry is the default retry condition,
// inspired by https://github.com/hashicorp/go-retryablehttp/blob/40b0cad1633fd521cee5884724fcf03d039aaf3f/client.go#L68-L86.
func DefaultRetry(resp *http.Response, respErr error) bool {
	if respErr != nil {
		switch errMsg := respErr.Error(); {
		case strings.Contains(errMsg, `redirects`):
			return false
		case strings.Contains(errMsg, `unsupported protocol scheme`):
			return false
		case strings.Contains(errMsg, `certificate is not trusted`):
			return false
		case strings.Contains(errMsg, `invalid header`):
			return false
		case strings.Contains(errMsg, `failed to verify certificate`):
			return false
		}

		// Retry if receiving connection closed.
		return true
	}

	// Retry if receiving rate-limited of server.
	if resp.StatusCode == http.StatusTooManyRequests {
		return true
	}

	// Retry if receiving unexpected responses.
	if resp.StatusCode == 0 || (resp.StatusCode >= 500 && resp.StatusCode != http.StatusNotImplemented) {
		return true
	}

	return false
}

// createRetryBackoff creates a backoff function for retry operation.
func createRetryBackoff(waitMin, waitMax time.Duration, attemptMax int) func(int, *http.Response) (time.Duration, bool) {
	return func(attemptNum int, resp *http.Response) (wait time.Duration, ok bool) {
		if attemptNum > attemptMax {
			return 0, false
		}

		if resp != nil && (resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable) {
			if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
				if seconds, err := strconv.Atoi(retryAfter); err == nil {
					return time.Duration(seconds) * time.Second, true
				}
			}
		}

		wait = time.Duration(math.Pow(2, float64(attemptNum)) * float64(waitMin))
		return min(wait, waitMax), true
	}
}
