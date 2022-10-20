package hooks

import (
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/defenseunicorns/zarf/src/internal/git"
	"github.com/defenseunicorns/zarf/src/internal/message"
)

const (
	noTransform = "/zarf-3xx-no-transform-%F0%9F%A6%84"
)

// HTTPProxy provides an http handler signature for proxying airgap requests.
func HTTPProxy(w http.ResponseWriter, r *http.Request) {
	zarfState, err := getStateFromAgentPod(zarfStatePath)
	if err != nil {
		message.Debugf("Unable to load the ZarfState file so that the Agent can mutate pods: %#v", err)
	}

	npmToken, err := os.ReadFile("/etc/zarf-npm/npm")
	if err != nil {
		message.Warnf("Unable to read the npmToken file within the zarf-agent pod.")
		npmToken = []byte(zarfState.GitServer.PushPassword)
	}

	director := func(req *http.Request) {
		message.Debugf("Before Req %#v", req)
		message.Debugf("Before Req URL %#v", req.URL)

		// We add this so that we can use it to rewrite urls in the response if needed
		req.Header.Add("X-Forwarded-Host", req.Host)

		// We remove this so that go will encode and decode on our behalf (see https://pkg.go.dev/net/http#Transport DisableCompression)
		req.Header.Del("Accept-Encoding")

		var targetURL *url.URL

		if strings.HasPrefix(req.UserAgent(), "git") {
			req.SetBasicAuth(zarfState.GitServer.PushUsername, zarfState.GitServer.PushPassword)

			if strings.HasPrefix(req.URL.Path, noTransform) {
				targetURL = noTransformTarget(zarfState.GitServer.Address, req.URL.Path)
			} else {
				// TODO: (@WSTARR) Remove hardcoded https, this doesn't come through on scheme, but we could check it from req.TLS (though we only serve https right now anyway)
				transformedURL, err := git.TransformURL(zarfState.GitServer.Address, "https://"+req.Host+req.URL.String(), zarfState.GitServer.PushUsername)
				message.Debugf("%#v", err)

				targetURL, err = url.Parse(transformedURL)
				message.Debugf("%#v", err)
			}
		} else {
			req.SetBasicAuth(zarfState.GitServer.PushUsername, zarfState.GitServer.PushPassword)

			if strings.HasPrefix(req.URL.Path, noTransform) {
				targetURL = noTransformTarget(zarfState.GitServer.Address, req.URL.Path)
			} else {
				transformedURL := ""

				if strings.HasPrefix(req.UserAgent(), "pip") || strings.HasPrefix(req.UserAgent(), "twine") {
					transformedURL = pipTransformURL(zarfState.GitServer.Address, "https://"+req.Host+req.URL.String(), zarfState.GitServer.PushUsername)
					// TODO: (@WSTARR) We will need to support separate git repos and package registries in the future
				} else if strings.HasPrefix(req.UserAgent(), "npm") || strings.HasPrefix(req.UserAgent(), "yarn") {
					transformedURL = npmTransformURL(zarfState.GitServer.Address, "https://"+req.Host+req.URL.String(), zarfState.GitServer.PushUsername)
					// TODO: (@WSTARR) We will need to support separate external git repos and package registries in the future
					req.Header.Set("Authorization", "Bearer "+string(npmToken))
				} else {
					transformedURL = genTransformURL(zarfState.GitServer.Address, "https://"+req.Host+req.URL.String(), zarfState.GitServer.PushUsername)
					// TODO: (@WSTARR) We will need to support separate external git repos and package registries in the future
				}

				targetURL, err = url.Parse(transformedURL)
				message.Debugf("%#v", err)
			}
		}

		req.Host = targetURL.Host
		req.URL = targetURL

		message.Debugf("After Req %#v", req)
		message.Debugf("After Req URL%#v", req.URL)
	}

	response := func(resp *http.Response) error {
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

		contentType := resp.Header.Get("Content-Type")
		message.Debugf("%s", contentType)

		// TODO: (@WSTARR) Refactor to be more concise/descriptive
		if strings.HasPrefix(resp.Header.Get("Content-Type"), "text") ||
			strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") ||
			strings.HasPrefix(resp.Header.Get("Content-Type"), "application/xml") {
			forwardedHost := resp.Request.Header.Get("X-Forwarded-Host")
			message.Debugf("%#v", err)
			body, err := io.ReadAll(resp.Body)
			message.Debugf("%#v", err)
			err = resp.Body.Close()
			message.Debugf("%#v", err)
			bodyString := string(body)
			message.Warnf("%s", bodyString)
			// TODO: (@WSTARR) Remove hardcoded https, this doesn't come through on scheme, but we could check it from resp.TLS (though we only serve https right now anyway)
			bodyString = strings.ReplaceAll(bodyString, zarfState.GitServer.Address, "https://"+forwardedHost+noTransform)
			message.Warnf("%s", bodyString)
			resp.Body = io.NopCloser(strings.NewReader(bodyString))
			resp.ContentLength = int64(len(bodyString))
			resp.Header.Set("Content-Length", fmt.Sprint(int64(len(bodyString))))
		}

		message.Debugf("After Resp %#v", resp)

		return nil
	}

	proxy := &httputil.ReverseProxy{Director: director, ModifyResponse: response}

	proxy.ServeHTTP(w, r)
}

func noTransformTarget(address string, path string) *url.URL {
	targetURL, err := url.Parse(address)
	message.Debugf("%#v", err)

	targetURL.Path = strings.TrimPrefix(path, noTransform)

	return targetURL
}

func npmTransformURL(baseURL string, url string, username string) string {
	// For further explanation: https://regex101.com/r/guYXVV/1
	// This regex was created with information from https://github.com/go-gitea/gitea/blob/0e58201d1a8247561809d832eb8f576e05e5d26d/routers/api/packages/api.go#L210
	npmURLRegex := regexp.MustCompile(`^(?P<proto>[a-z]+:\/\/)(?P<hostPath>.+?)\/` +
		`(?P<npmPath>((@[\w\.\-\~]+\/)?[\w\.\-\~]+(\/-\/([\w\.\-\~]+\/)?[\w\.\-\~]+\.[\w]+)?(\/-rev\/.+)?)|(-\/(npm|v1|user|package)\/.+))$`)

	matches := npmURLRegex.FindStringSubmatch(url)
	idx := npmURLRegex.SubexpIndex

	if len(matches) == 0 {
		// Unable to find a substring match for the regex
		message.Debugf("unable to extract the npm url from the url %s", url)
	}

	// TODO: (@WSTARR) %s/api/packages/%s is very Gitea specific but with a config option this could be adapted to GitLab easily
	// transformedURL := fmt.Sprintf("%s/api/v4/projects/39799997/packages/npm/%s", baseURL, matches[idx("npmPath")])
	transformedURL := fmt.Sprintf("%s/api/packages/%s/npm/%s", baseURL, username, matches[idx("npmPath")])

	return transformedURL
}

func pipTransformURL(baseURL string, url string, username string) string {
	// For further explanation: https://regex101.com/r/lreZiD/1
	// This regex was created with information from https://github.com/go-gitea/gitea/blob/0e58201d1a8247561809d832eb8f576e05e5d26d/routers/api/packages/api.go#L210
	pipURLRegex := regexp.MustCompile(`^(?P<proto>[a-z]+:\/\/)(?P<hostPath>.+?)(?P<pipPath>(\/(simple|files\/)[\/\w\-\.\?\=&%#]*?))?$`)

	matches := pipURLRegex.FindStringSubmatch(url)
	idx := pipURLRegex.SubexpIndex

	if len(matches) == 0 {
		// Unable to find a substring match for the regex
		message.Debugf("unable to extract the pip url from the url %s", url)
	}

	// TODO: (@WSTARR) %s/api/packages/%s is very Gitea specific but with a config option this could be adapted to GitLab easily
	// transformedURL := fmt.Sprintf("%s/api/v4/projects/39799997/packages/pypi%s", baseURL, matches[idx("pipPath")])
	transformedURL := fmt.Sprintf("%s/api/packages/%s/pypi%s", baseURL, username, matches[idx("pipPath")])

	return transformedURL
}

func genTransformURL(baseURL string, url string, username string) string {
	// For further explanation: https://regex101.com/r/qcg6Gr/1
	genURLRegex := regexp.MustCompile(`^(?P<proto>[a-z]+:\/\/)(?P<host>.+?)(?P<startPath>\/[\w\-\.%]+?\/[\w\-\.%]+?)?(?P<midPath>\/.+?)??(?P<version>\/[\w\-\.%]+?)?(?P<package>\/[\w\-\.\?\=&%#]+?)$`)

	matches := genURLRegex.FindStringSubmatch(url)
	idx := genURLRegex.SubexpIndex

	if len(matches) == 0 {
		// Unable to find a substring match for the regex
		message.Debugf("unable to extract the generic url from the url %s", url)
	}

	packageName := matches[idx("startPath")]
	// NOTE: We remove the protocol and file name that https://zarf.dev/package/package1.zip and http://zarf.dev/package/package2.zip
	// resolve to the same folder (as they would in real life)
	sanitizedURL := fmt.Sprintf("%s%s%s", matches[idx("host")], matches[idx("startPath")], matches[idx("midPath")])

	// Add crc32 hash of the url to the end of the package name
	table := crc32.MakeTable(crc32.IEEE)
	checksum := crc32.Checksum([]byte(sanitizedURL), table)
	packageName = fmt.Sprintf("%s-%d", strings.ReplaceAll(packageName, "/", ""), checksum)

	version := matches[idx("version")]
	if version == "" {
		version = matches[idx("package")]
	}

	// TODO: (@WSTARR) %s/api/packages/%s is very Gitea specific but with a config option this could be adapted to GitLab easily
	// transformedURL := fmt.Sprintf("%s/api/v4/projects/39799997/packages/generic/%s%s%s", baseURL, packageName, version, matches[idx("package")])
	transformedURL := fmt.Sprintf("%s/api/packages/%s/generic/%s%s%s", baseURL, username, packageName, version, matches[idx("package")])

	return transformedURL
}
