// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package http provides a http server for the webhook and proxy.
package http

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/types"
)

// ProxyHandler constructs a new httputil.ReverseProxy and returns an http handler.
func ProxyHandler(cluster *cluster.Cluster) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state, err := cluster.LoadZarfState(r.Context())
		if err != nil {
			message.Debugf("%#v", err)
			w.WriteHeader(http.StatusInternalServerError)
			//nolint: errcheck // ignore
			w.Write([]byte("unable to load Zarf state, see the Zarf HTTP proxy logs for more details"))
			return
		}
		err = proxyRequestTransform(r, state)
		if err != nil {
			message.Debugf("%#v", err)
			w.WriteHeader(http.StatusInternalServerError)
			//nolint: errcheck // ignore
			w.Write([]byte("unable to transform the provided request, see the Zarf HTTP proxy logs for more details"))
			return
		}
		proxy := &httputil.ReverseProxy{Director: func(_ *http.Request) {}, ModifyResponse: proxyResponseTransform}
		proxy.ServeHTTP(w, r)
	}
}

func proxyRequestTransform(r *http.Request, state *types.ZarfState) error {
	// We add this so that we can use it to rewrite urls in the response if needed
	r.Header.Add("X-Forwarded-Host", r.Host)

	// We remove this so that go will encode and decode on our behalf (see https://pkg.go.dev/net/http#Transport DisableCompression)
	r.Header.Del("Accept-Encoding")

	// Setup authentication for each type of service based on User Agent
	switch {
	case isGitUserAgent(r.UserAgent()):
		r.SetBasicAuth(state.GitServer.PushUsername, state.GitServer.PushPassword)
	case isNpmUserAgent(r.UserAgent()):
		r.Header.Set("Authorization", "Bearer "+state.ArtifactServer.PushToken)
	default:
		r.SetBasicAuth(state.ArtifactServer.PushUsername, state.ArtifactServer.PushToken)
	}

	// Transform the URL; if we see the NoTransform prefix, strip it; otherwise, transform the URL based on User Agent
	var err error
	var targetURL *url.URL
	if strings.HasPrefix(r.URL.Path, transform.NoTransform) {
		switch {
		case isGitUserAgent(r.UserAgent()):
			targetURL, err = transform.NoTransformTarget(state.GitServer.Address, r.URL.Path)
		default:
			targetURL, err = transform.NoTransformTarget(state.ArtifactServer.Address, r.URL.Path)
		}
	} else {
		switch {
		case isGitUserAgent(r.UserAgent()):
			targetURL, err = transform.GitURL(state.GitServer.Address, getTLSScheme(r.TLS)+r.Host+r.URL.String(), state.GitServer.PushUsername)
		case isPipUserAgent(r.UserAgent()):
			targetURL, err = transform.PipTransformURL(state.ArtifactServer.Address, getTLSScheme(r.TLS)+r.Host+r.URL.String())
		case isNpmUserAgent(r.UserAgent()):
			targetURL, err = transform.NpmTransformURL(state.ArtifactServer.Address, getTLSScheme(r.TLS)+r.Host+r.URL.String())
		default:
			targetURL, err = transform.GenTransformURL(state.ArtifactServer.Address, getTLSScheme(r.TLS)+r.Host+r.URL.String())
		}
	}
	if err != nil {
		return err
	}

	r.Host = targetURL.Host
	r.URL = targetURL
	r.RequestURI = getRequestURI(targetURL.Path, targetURL.RawQuery, targetURL.Fragment)

	return nil
}

func proxyResponseTransform(resp *http.Response) error {
	// Handle redirection codes (3xx) by adding a marker to let Zarf know this has been redirected
	if resp.StatusCode/100 == 3 {
		locationURL, err := url.Parse(resp.Header.Get("Location"))
		if err != nil {
			return err
		}
		locationURL.Path = transform.NoTransform + locationURL.Path
		locationURL.Host = resp.Request.Header.Get("X-Forwarded-Host")

		resp.Header.Set("Location", locationURL.String())
	}

	// Handle text content returns that may contain links
	contentType := resp.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "text") || strings.HasPrefix(contentType, "application/json") || strings.HasPrefix(contentType, "application/xml") {
		forwardedPrefix := fmt.Sprintf("%s%s%s", getTLSScheme(resp.Request.TLS), resp.Request.Header.Get("X-Forwarded-Host"), transform.NoTransform)
		targetPrefix := fmt.Sprintf("%s%s", getTLSScheme(resp.TLS), resp.Request.Host)
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		err = resp.Body.Close()
		if err != nil {
			return err
		}
		bodyString := strings.ReplaceAll(string(b), targetPrefix, forwardedPrefix)

		resp.Body = io.NopCloser(strings.NewReader(bodyString))
		resp.ContentLength = int64(len(bodyString))
		resp.Header.Set("Content-Length", fmt.Sprint(int64(len(bodyString))))
	}
	return nil
}

func getTLSScheme(tls *tls.ConnectionState) string {
	scheme := "https://"
	if tls == nil {
		scheme = "http://"
	}
	return scheme
}

func getRequestURI(path, query, fragment string) string {
	uri := path
	if query != "" {
		uri += "?" + query
	}
	if fragment != "" {
		uri += "#" + fragment
	}
	return uri
}

func isGitUserAgent(userAgent string) bool {
	return strings.HasPrefix(userAgent, "git")
}

func isPipUserAgent(userAgent string) bool {
	return strings.HasPrefix(userAgent, "pip") || strings.HasPrefix(userAgent, "twine")
}

func isNpmUserAgent(userAgent string) bool {
	return strings.HasPrefix(userAgent, "npm") || strings.HasPrefix(userAgent, "pnpm") || strings.HasPrefix(userAgent, "yarn") || strings.HasPrefix(userAgent, "bun")
}
