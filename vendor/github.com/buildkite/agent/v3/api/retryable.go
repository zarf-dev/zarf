package api

import (
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"syscall"
)

var retrableErrorSuffixes = []string{
	syscall.ECONNREFUSED.Error(),
	syscall.ECONNRESET.Error(),
	syscall.ETIMEDOUT.Error(),
	"no such host",
	"remote error: handshake failure",
	io.ErrUnexpectedEOF.Error(),
	io.EOF.Error(),
}

var retryableStatuses = []int{
	http.StatusTooManyRequests,     // 429
	http.StatusInternalServerError, // 500
	http.StatusBadGateway,          // 502
	http.StatusServiceUnavailable,  // 503
	http.StatusGatewayTimeout,      // 504
}

// IsRetryableStatus returns true if the response's StatusCode is one that we should retry.
func IsRetryableStatus(r *Response) bool {
	return r.StatusCode >= 400 && slices.Contains(retryableStatuses, r.StatusCode)
}

// Looks at a bunch of connection related errors, and returns true if the error
// matches one of them.
func IsRetryableError(err error) bool {
	if neterr, ok := err.(net.Error); ok {
		if neterr.Temporary() {
			return true
		}
	}

	if neterr, ok := err.(net.Error); ok && neterr.Timeout() {
		return true
	}

	if urlerr, ok := err.(*url.Error); ok {
		if strings.Contains(urlerr.Error(), "use of closed network connection") {
			return true
		}

		if neturlerr, ok := urlerr.Err.(net.Error); ok && neturlerr.Timeout() {
			return true
		}
	}

	if strings.Contains(err.Error(), "request canceled while waiting for connection") {
		return true
	}

	s := err.Error()
	for _, suffix := range retrableErrorSuffixes {
		if strings.HasSuffix(s, suffix) {
			return true
		}
	}

	return false
}
