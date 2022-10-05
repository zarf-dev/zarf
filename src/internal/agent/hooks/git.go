package hooks

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/defenseunicorns/zarf/src/internal/git"
	"github.com/defenseunicorns/zarf/src/internal/message"
)

const (
	noTransform = "/zarf-3xx-no-transform-%F0%9F%A6%84"
)

// GitProxy provides an http handler signature for proxying git repo requests.
func GitProxy(w http.ResponseWriter, r *http.Request) {
	zarfState, err := getStateFromAgentPod(zarfStatePath)
	if err != nil {
		message.Debugf("Unable to load the ZarfState file so that the Agent can mutate pods: %#v", err)
	}

	directorGit := func(req *http.Request) {

		message.Debugf("Before Req %#v", req)
		message.Debugf("Before Req URL %#v", req.URL)

		req.Header.Add("X-Forwarded-Host", req.Host)

		if strings.HasPrefix(req.URL.Path, noTransform) {
			targetURL, err := url.Parse(zarfState.GitServer.Address)
			message.Debugf("%#v", err)

			req.Host = targetURL.Host
			req.URL.Scheme = targetURL.Scheme
			req.URL.Host = targetURL.Host
			req.URL.Path = strings.TrimPrefix(req.URL.Path, noTransform)
		} else {
			// TODO: Remove hardcoded https, this doesn't come through on scheme, but we would expect to receive (pretty much always) https reqs
			transformedURL, err := git.TransformURL(zarfState.GitServer.Address, "https://"+req.Host+req.URL.String(), zarfState.GitServer.PullUsername)
			message.Debugf("%#v", err)

			targetURL, err := url.Parse(transformedURL)
			message.Debugf("%#v", err)

			req.Host = targetURL.Host
			req.URL.Scheme = targetURL.Scheme
			req.URL.Host = targetURL.Host
			req.URL.Path = targetURL.Path
		}

		req.SetBasicAuth(zarfState.GitServer.PullUsername, zarfState.GitServer.PullPassword)

		message.Debugf("After Req %#v", req)
		message.Debugf("After Req URL%#v", req.URL)
	}

	responseGit := func(resp *http.Response) error {
		message.Debugf("Before Resp %#v", resp)

		// Handle redirection codes (3xx) by adding a marker to let Zarf know this has been redirected
		if resp.StatusCode/100 == 3 {
			message.Debugf("Before Resp Location %#v", resp.Header.Get("Location"))

			locationURL, err := url.Parse(resp.Header.Get("Location"))
			message.Debugf("%#v", err)
			locationURL.Path = noTransform + locationURL.Path
			locationURL.Host = resp.Request.Header.Get("X-Forwarded-Host")

			resp.Header.Set("Location", locationURL.String())

			message.Debugf("After Resp Location %#v", resp.Header.Get("Location"))
		}

		message.Debugf("After Resp %#v", resp)

		return nil
	}

	proxy := &httputil.ReverseProxy{Director: directorGit, ModifyResponse: responseGit}

	proxy.ServeHTTP(w, r)
}
