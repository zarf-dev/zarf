package api

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"

	"github.com/buildkite/roko"
)

var retriableErrorSuffixes = []string{
	syscall.ECONNREFUSED.Error(),
	syscall.ECONNRESET.Error(),
	syscall.ETIMEDOUT.Error(),
	"no such host",
	"remote error: handshake failure",
	io.ErrUnexpectedEOF.Error(),
	io.EOF.Error(),
}

// IsRetryableStatus returns true if the response's StatusCode is one that we should retry.
// Success statuses (2xx) are not considered retryable — they are not errors.
func IsRetryableStatus(r *Response) bool {
	switch {
	case r.StatusCode == http.StatusTooManyRequests:
		return true
	case r.StatusCode >= 500:
		return true
	default:
		return false
	}
}

// Looks at a bunch of connection related errors, and returns true if the error
// matches one of them.
func IsRetryableError(err error) bool {
	var neterr net.Error
	if errors.As(err, &neterr) {
		if neterr.Timeout() {
			return true
		}
	}

	var urlerr *url.Error
	if errors.As(err, &urlerr) {
		if strings.Contains(urlerr.Error(), "use of closed network connection") {
			return true
		}
	}

	if strings.Contains(err.Error(), "request canceled while waiting for connection") {
		return true
	}

	s := err.Error()
	for _, suffix := range retriableErrorSuffixes {
		if strings.HasSuffix(s, suffix) {
			return true
		}
	}

	return false
}

// BreakOnNonRetryable calls r.Break() if the error from an API call is not
// worth retrying. An error is retryable if the response has a retryable status
// code (429, 5xx) or if there was no response and the error is a retryable
// network-level error (connection reset, timeout, etc.). All other errors
// — including all non-429 4xx status codes — cause a break.
//
// This should be called inside roko retry callbacks after every API call.
// If err is nil, this is a no-op.
// BreakOnNonRetryable returns true if it called r.Break() (i.e. the error is
// non-retryable). Callers can use this to avoid logging misleading retry
// information when the retrier is about to give up.
func BreakOnNonRetryable(r *roko.Retrier, resp *Response, err error) (broke bool) {
	if err == nil {
		return false
	}
	if resp != nil {
		if !IsRetryableStatus(resp) {
			r.Break()
			return true
		}
		return false
	}
	if !IsRetryableError(err) {
		r.Break()
		return true
	}
	return false
}
