package http

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/defenseunicorns/zarf/src/internal/agent/proxy"
	"github.com/defenseunicorns/zarf/src/internal/git"
	"github.com/defenseunicorns/zarf/src/internal/message"
)

// ProxyHandler constructs a new httputil.ReverseProxy and returns an http handler.
func ProxyHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		proxy := &httputil.ReverseProxy{Director: proxyDirector, ModifyResponse: proxyResponse}
		proxy.ServeHTTP(w, r)
	}
}

func proxyDirector(req *http.Request) {
	message.Debugf("Before Req %#v", req)
	message.Debugf("Before Req URL %#v", req.URL)

	// We add this so that we can use it to rewrite urls in the response if needed
	req.Header.Add("X-Forwarded-Host", req.Host)

	// We remove this so that go will encode and decode on our behalf (see https://pkg.go.dev/net/http#Transport DisableCompression)
	req.Header.Del("Accept-Encoding")

	zarfState, npmToken := proxy.GetProxyState()

	// Setup authentication for the given service
	if isNpmUserAgent(req.UserAgent()) {
		req.Header.Set("Authorization", "Bearer "+npmToken)
	} else {
		req.SetBasicAuth(zarfState.GitServer.PushUsername, zarfState.GitServer.PushPassword)
	}

	var targetURL *url.URL
	var transformedURL string
	var err error

	// If we see the NoTranform prefix, just strip it otherwise, transform the URL based on User Agent
	if strings.HasPrefix(req.URL.Path, proxy.NoTransform) {
		if targetURL, err = proxy.NoTransformTarget(zarfState.GitServer.Address, req.URL.Path); err != nil {
			message.Debugf("%#v", err)
		}
	} else {
		switch {
		case isGitUserAgent(req.UserAgent()):
			// TODO: (@WSTARR) Remove hardcoded https from these, this doesn't come through on scheme, but we could check it from req.TLS (though we only serve https right now anyway)
			transformedURL, err = git.TransformURL(zarfState.GitServer.Address, "https://"+req.Host+req.URL.String(), zarfState.GitServer.PushUsername)
		case isPipUserAgent(req.UserAgent()):
			transformedURL, err = proxy.PipTransformURL(zarfState.GitServer.Address, "https://"+req.Host+req.URL.String(), zarfState.GitServer.PushUsername)
		case isNpmUserAgent(req.UserAgent()):
			transformedURL, err = proxy.NpmTransformURL(zarfState.GitServer.Address, "https://"+req.Host+req.URL.String(), zarfState.GitServer.PushUsername)
		default:
			transformedURL, err = proxy.GenTransformURL(zarfState.GitServer.Address, "https://"+req.Host+req.URL.String(), zarfState.GitServer.PushUsername)
		}

		if err != nil {
			message.Debugf("%#v", err)
		}

		if targetURL, err = url.Parse(transformedURL); err != nil {
			message.Debugf("%#v", err)
		}
	}

	req.Host = targetURL.Host
	req.URL = targetURL

	message.Debugf("After Req %#v", req)
	message.Debugf("After Req URL%#v", req.URL)
}

func proxyResponse(resp *http.Response) error {
	message.Debugf("Before Resp %#v", resp)

	zarfState, _ := proxy.GetProxyState()

	// Handle redirection codes (3xx) by adding a marker to let Zarf know this has been redirected
	if resp.StatusCode/100 == 3 {
		message.Debugf("Before Resp Location %#v", resp.Header.Get("Location"))

		locationURL, err := url.Parse(resp.Header.Get("Location"))
		message.Debugf("%#v", err)
		locationURL.Path = proxy.NoTransform + locationURL.Path
		locationURL.Host = resp.Request.Header.Get("X-Forwarded-Host")

		resp.Header.Set("Location", locationURL.String())

		message.Debugf("After Resp Location %#v", resp.Header.Get("Location"))
	}

	contentType := resp.Header.Get("Content-Type")

	// TODO: (@WSTARR) Refactor to be more concise/descriptive
	if strings.HasPrefix(contentType, "text") || strings.HasPrefix(contentType, "application/json") || strings.HasPrefix(contentType, "application/xml") {
		message.Debugf("Resp Request: %#v", resp.Request)
		forwardedHost := resp.Request.Header.Get("X-Forwarded-Host")
		body, err := io.ReadAll(resp.Body)
		message.Debugf("%#v", err)
		err = resp.Body.Close()
		message.Debugf("%#v", err)
		bodyString := string(body)
		message.Warnf("%s", bodyString)
		// TODO: (@WSTARR) Remove hardcoded https, this doesn't come through on scheme, but we could check it from resp.TLS (though we only serve https right now anyway)
		// TODO: (@WSTARR) This is also our only use of state in the response.  We should likely just use the request object instead
		bodyString = strings.ReplaceAll(bodyString, zarfState.GitServer.Address, "https://"+forwardedHost+proxy.NoTransform)
		message.Warnf("%s", bodyString)
		resp.Body = io.NopCloser(strings.NewReader(bodyString))
		resp.ContentLength = int64(len(bodyString))
		resp.Header.Set("Content-Length", fmt.Sprint(int64(len(bodyString))))
	}

	message.Debugf("After Resp %#v", resp)

	return nil
}

func isGitUserAgent(userAgent string) bool {
	return strings.HasPrefix(userAgent, "git")
}

func isPipUserAgent(userAgent string) bool {
	return strings.HasPrefix(userAgent, "pip") || strings.HasPrefix(userAgent, "twine")
}

func isNpmUserAgent(userAgent string) bool {
	return strings.HasPrefix(userAgent, "npm") || strings.HasPrefix(userAgent, "yarn")
}
