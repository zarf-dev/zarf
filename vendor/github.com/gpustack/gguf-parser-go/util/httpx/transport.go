package httpx

import (
	"net/http"
)

// DefaultTransport is similar to the default http.DefaultTransport used by the package.
var DefaultTransport http.RoundTripper = Transport()

// DefaultInsecureTransport is the default http.DefaultTransport used by the package,
// with TLS insecure skip verify.
var DefaultInsecureTransport http.RoundTripper = Transport(TransportOptions().WithoutInsecureVerify())

// Transport returns a new http.Transport with the given options,
// the result http.Transport is used for constructing http.Client.
func Transport(opts ...*TransportOption) *http.Transport {
	var o *TransportOption
	if len(opts) > 0 {
		o = opts[0]
	} else {
		o = TransportOptions()
	}

	return o.transport
}
