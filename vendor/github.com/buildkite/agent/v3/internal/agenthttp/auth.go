package agenthttp

import (
	"fmt"
	"net/http"
)

// authenticatedTransport manages injection of the API token into every request.
// Using a transport to inject credentials into every request like this is
// ugly because http.RoundTripper has specific requirements, but has
// precedent (e.g. https://github.com/golang/oauth2/blob/master/transport.go).
type authenticatedTransport struct {
	// If set, the header "Authorization: Token %s" will be added to all requests.
	// Mutually incompatible with Bearer.
	Token string

	// If set, the header "Authorization: Bearer %s" will be added to all requests.
	// Mutually incompatible with Token.
	Bearer string

	// Delegate is the underlying HTTP transport
	Delegate http.RoundTripper
}

// RoundTrip invoked each time a request is made.
func (t authenticatedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Per net/http#RoundTripper:
	//
	// "RoundTrip must always close the body, including on errors, ..."
	reqBodyClosed := false
	if req.Body != nil {
		defer func() {
			if !reqBodyClosed {
				req.Body.Close() //nolint:errcheck // req.Body is only used in a read-only manner.
			}
		}()
	}

	if t.Token == "" && t.Bearer == "" {
		return nil, fmt.Errorf("Invalid token, empty string supplied")
	}

	// Per net/http#RoundTripper:
	//
	// "RoundTrip should not modify the request, except for
	// consuming and closing the Request's Body. RoundTrip may
	// read fields of the request in a separate goroutine. Callers
	// should not mutate or reuse the request until the Response's
	// Body has been closed."
	//
	// But we can pass a _different_ request to t.Delegate.RoundTrip.
	// req.Clone does a sufficiently deep clone (including Header which we
	// modify).
	req = req.Clone(req.Context())
	switch {
	case t.Token != "":
		req.Header.Set("Authorization", "Token "+t.Token)
	case t.Bearer != "":
		req.Header.Set("Authorization", "Bearer "+t.Bearer)
	}

	// req.Body is assumed to be closed by the delegate.
	reqBodyClosed = true
	return t.Delegate.RoundTrip(req)
}

// CancelRequest forwards the call to t.Delegate, if it implements CancelRequest
// itself.
func (t *authenticatedTransport) CancelRequest(req *http.Request) {
	canceler, ok := t.Delegate.(interface{ CancelRequest(*http.Request) })
	if !ok {
		return
	}
	canceler.CancelRequest(req)
}

// CloseIdleConnections forwards the call to t.Delegate, if it implements
// CloseIdleConnections itself.
func (t *authenticatedTransport) CloseIdleConnections() {
	closer, ok := t.Delegate.(interface{ CloseIdleConnections() })
	if !ok {
		return
	}
	closer.CloseIdleConnections()
}
