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
	"github.com/defenseunicorns/zarf/src/internal/agent/state"
	"github.com/defenseunicorns/zarf/src/internal/packager/git"
	"github.com/defenseunicorns/zarf/src/pkg/message"
)

// ProxyHandler constructs a new httputil.ReverseProxy and returns an http handler.
func ProxyHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := setReqURL(r)
		if err != nil {
			message.Debugf("%#v", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(fmt.Sprintf("%#v", err)))
			return
		}

		proxy := &httputil.ReverseProxy{ModifyResponse: proxyResponse}
		proxy.ServeHTTP(w, r)
	}
}

func setReqURL(r *http.Request) error {
	message.Debugf("Before Req %#v", r)
	message.Debugf("Before Req URL %#v", r.URL)

	// We add this so that we can use it to rewrite urls in the response if needed
	r.Header.Add("X-Forwarded-Host", r.Host)

	// We remove this so that go will encode and decode on our behalf (see https://pkg.go.dev/net/http#Transport DisableCompression)
	r.Header.Del("Accept-Encoding")

	zarfState, err := state.GetZarfStateFromAgentPod()
	if err != nil {
		return err
	}

	var targetURL *url.URL

	// If 'git' is the username use the configured git server, otherwise use the artifact server
	if isGitUserAgent(r.UserAgent()) {
		// If we see the NoTransform prefix, just strip it otherwise, transform the URL based on User Agent
		if strings.HasPrefix(r.URL.Path, proxy.NoTransform) {
			targetURL, err = proxy.NoTransformTarget(zarfState.GitServer.Address, r.URL.Path)
		} else {
			g := git.New(zarfState.GitServer)

			var transformedURL string
			transformedURL, err = g.TransformURL(getTLSScheme(r.TLS) + r.Host + r.URL.String())
			if err != nil {
				return err
			}
			targetURL, err = url.Parse(transformedURL)
			r.SetBasicAuth(zarfState.GitServer.PushUsername, zarfState.GitServer.PushPassword)
		}
	} else {
		// If we see the NoTransform prefix, just strip it otherwise, transform the URL based on User Agent
		if strings.HasPrefix(r.URL.Path, proxy.NoTransform) {
			targetURL, err = proxy.NoTransformTarget(zarfState.ArtifactServer.Address, r.URL.Path)
		} else {
			switch {
			case isPipUserAgent(r.UserAgent()):
				targetURL, err = proxy.PipTransformURL(zarfState.ArtifactServer.Address, getTLSScheme(r.TLS)+r.Host+r.URL.String(), zarfState.ArtifactServer.PushUsername)
				r.SetBasicAuth(zarfState.ArtifactServer.PushUsername, zarfState.ArtifactServer.PushToken)
			case isNpmUserAgent(r.UserAgent()):
				targetURL, err = proxy.NpmTransformURL(zarfState.ArtifactServer.Address, getTLSScheme(r.TLS)+r.Host+r.URL.String(), zarfState.ArtifactServer.PushUsername)
				r.Header.Set("Authorization", "Bearer "+zarfState.ArtifactServer.PushToken)
			default:
				targetURL, err = proxy.GenTransformURL(zarfState.ArtifactServer.Address, getTLSScheme(r.TLS)+r.Host+r.URL.String(), zarfState.ArtifactServer.PushUsername)
				r.SetBasicAuth(zarfState.ArtifactServer.PushUsername, zarfState.ArtifactServer.PushToken)
			}
		}
	}

	if err != nil {
		return err
	}

	r.Host = targetURL.Host
	r.URL = targetURL

	message.Debugf("After Req %#v", r)
	message.Debugf("After Req URL%#v", r.URL)

	return nil
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
