package http

import (
	"crypto/tls"
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

	// TODO: (@WSTARR) we will eventually need to support a separate git host and package registry host (potential to expand the NPM job)
	zarfState, npmToken, err := proxy.GetProxyState()
	if err != nil {
		message.Debugf("%#v", err)
	}

	// Setup authentication for the given service
	if isNpmUserAgent(req.UserAgent()) {
		req.Header.Set("Authorization", "Bearer "+npmToken)
	} else {
		req.SetBasicAuth(zarfState.GitServer.PushUsername, zarfState.GitServer.PushPassword)
	}

	var targetURL *url.URL
	var transformedURL string

	// If we see the NoTransform prefix, just strip it otherwise, transform the URL based on User Agent
	if strings.HasPrefix(req.URL.Path, proxy.NoTransform) {
		if targetURL, err = proxy.NoTransformTarget(zarfState.GitServer.Address, req.URL.Path); err != nil {
			message.Debugf("%#v", err)
		}
	} else {
		switch {
		case isGitUserAgent(req.UserAgent()):
			transformedURL, err = git.TransformURL(zarfState.GitServer.Address, getTLSScheme(req.TLS)+req.Host+req.URL.String(), zarfState.GitServer.PushUsername)
		case isPipUserAgent(req.UserAgent()):
			transformedURL, err = proxy.PipTransformURL(zarfState.GitServer.Address, getTLSScheme(req.TLS)+req.Host+req.URL.String(), zarfState.GitServer.PushUsername)
		case isNpmUserAgent(req.UserAgent()):
			transformedURL, err = proxy.NpmTransformURL(zarfState.GitServer.Address, getTLSScheme(req.TLS)+req.Host+req.URL.String(), zarfState.GitServer.PushUsername)
		default:
			transformedURL, err = proxy.GenTransformURL(zarfState.GitServer.Address, getTLSScheme(req.TLS)+req.Host+req.URL.String(), zarfState.GitServer.PushUsername)
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

	// Handle text content returns that may contain links
	if strings.HasPrefix(contentType, "text") || strings.HasPrefix(contentType, "application/json") || strings.HasPrefix(contentType, "application/xml") {
		err := replaceBodyLinks(resp)

		if err != nil {
			message.Debugf("%#v", err)
		}
	}

	message.Debugf("After Resp %#v", resp)

	return nil
}

func replaceBodyLinks(resp *http.Response) error {
	message.Debugf("Resp Request: %#v", resp.Request)

	// Create the forwarded (online) and target (offline) URL prefixes to replace
	forwardedPrefix := fmt.Sprintf("%s%s%s", getTLSScheme(resp.Request.TLS), resp.Request.Header.Get("X-Forwarded-Host"), proxy.NoTransform)
	targetPrefix := fmt.Sprintf("%s%s", getTLSScheme(resp.TLS), resp.Request.Host)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	err = resp.Body.Close()
	if err != nil {
		return err
	}

	bodyString := string(body)
	message.Warnf("%s", bodyString)

	bodyString = strings.ReplaceAll(bodyString, targetPrefix, forwardedPrefix)

	message.Warnf("%s", bodyString)

	// Setup the new reader, and correct the content length
	resp.Body = io.NopCloser(strings.NewReader(bodyString))
	resp.ContentLength = int64(len(bodyString))
	resp.Header.Set("Content-Length", fmt.Sprint(int64(len(bodyString))))

	return nil
}

func getTLSScheme(tls *tls.ConnectionState) string {
	scheme := "https://"

	if tls == nil {
		scheme = "http://"
	}

	return scheme
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
