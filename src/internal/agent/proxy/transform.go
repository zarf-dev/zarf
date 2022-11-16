package proxy

import (
	"fmt"
	"hash/crc32"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/defenseunicorns/zarf/src/internal/agent/state"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/types"
)

const (
	// NoTransform is the URL prefix added to HTTP 3xx or text URLs that instructs Zarf not to transform on a subsequent request.
	NoTransform = "/zarf-3xx-no-transform"
)

// GetProxyState gets the ZarfState and the NPM token for the proxy to use for a request.
func GetProxyState() (types.ZarfState, string, error) {
	zarfState, err := state.GetZarfStateFromAgentPod()
	if err != nil {
		return types.ZarfState{}, "", err
	}

	npmToken, err := os.ReadFile("/etc/zarf-npm/npm")
	if err != nil {
		message.Warnf("Unable to read the npmToken file within the zarf-agent pod, falling back to git password")
		npmToken = []byte(zarfState.GitServer.PushPassword)
	}

	return zarfState, string(npmToken), nil
}

// NoTransformTarget takes an address that Zarf should not transform, and removes the NoTransform prefix.
func NoTransformTarget(address string, path string) (*url.URL, error) {
	targetURL, err := url.Parse(address)
	if err != nil {
		return nil, err
	}

	targetURL.Path = strings.TrimPrefix(path, NoTransform)

	return targetURL, nil
}

// NpmTransformURL finds the npm API path on a given URL and transforms that to align with the offline registry.
func NpmTransformURL(baseURL string, reqURL string, username string) (string, error) {
	// For further explanation: https://regex101.com/r/RRyazc/2
	// This regex was created with information from https://github.com/go-gitea/gitea/blob/0e58201d1a8247561809d832eb8f576e05e5d26d/routers/api/packages/api.go#L210
	npmURLRegex := regexp.MustCompile(`^(?P<proto>[a-z]+:\/\/)(?P<hostPath>.+?)` +
		`(?P<npmPath>(\/(@[\w\.\-\~]+(\/|%2[fF]))?[\w\.\-\~]+(\/-\/([\w\.\-\~]+\/)?[\w\.\-\~]+\.[\w]+)?(\/-rev\/.+)?)|(-\/(npm|v1|user|package)\/.+))$`)

	return transformRegistryPath(baseURL, reqURL, username, npmURLRegex, "npmPath", "npm")
}

// PipTransformURL finds the pip API path on a given URL and transforms that to align with the offline registry.
func PipTransformURL(baseURL string, reqURL string, username string) (string, error) {
	// For further explanation: https://regex101.com/r/lreZiD/1
	// This regex was created with information from https://github.com/go-gitea/gitea/blob/0e58201d1a8247561809d832eb8f576e05e5d26d/routers/api/packages/api.go#L210
	pipURLRegex := regexp.MustCompile(`^(?P<proto>[a-z]+:\/\/)(?P<hostPath>.+?)(?P<pipPath>(\/(simple|files\/)[\/\w\-\.\?\=&%#]*?))?$`)

	return transformRegistryPath(baseURL, reqURL, username, pipURLRegex, "pipPath", "pypi")
}

// GenTransformURL finds the generic API path on a given URL and transforms that to align with the offline registry.
func GenTransformURL(baseURL string, reqURL string, username string) (string, error) {
	// For further explanation: https://regex101.com/r/qcg6Gr/1
	genURLRegex := regexp.MustCompile(`^(?P<proto>[a-z]+:\/\/)(?P<host>.+?)(?P<startPath>\/[\w\-\.%]+?\/[\w\-\.%]+?)?(?P<midPath>\/.+?)??(?P<version>\/[\w\-\.%]+?)?(?P<package>\/[\w\-\.\?\=&%#]+?)$`)

	matches := genURLRegex.FindStringSubmatch(reqURL)
	idx := genURLRegex.SubexpIndex

	if len(matches) == 0 {
		// Unable to find a substring match for the regex
		return "", fmt.Errorf("unable to extract the genericPath from the url %s", reqURL)
	}

	packageName := matches[idx("startPath")]
	// NOTE: We remove the protocol and file name so that https://zarf.dev/package/package1.zip and http://zarf.dev/package/package2.zip
	// resolve to the same "folder" (as they would in real life)
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

	return transformedURL, nil
}

// transformRegistryPath transforms a given request path using a new base URL, username and regex.
// - pathGroup specifies the named group for the registry's URL path inside the regex (i.e. pipPath) and registryType specifies the registry type (i.e. pypi).
func transformRegistryPath(baseURL string, reqURL string, username string, regex *regexp.Regexp, pathGroup string, registryType string) (string, error) {
	matches := regex.FindStringSubmatch(reqURL)
	idx := regex.SubexpIndex

	if len(matches) == 0 {
		// Unable to find a substring match for the regex
		return "", fmt.Errorf("unable to extract the %s from the url %s", pathGroup, reqURL)
	}

	// TODO: (@WSTARR) %s/api/packages/%s is very Gitea specific but with a config option this could be adapted to GitLab easily
	// transformedURL := fmt.Sprintf("%s/api/v4/projects/39799997/packages/%s%s", baseURL, regType, matches[idx("pipPath")])
	return fmt.Sprintf("%s/api/packages/%s/%s%s", baseURL, username, registryType, matches[idx(pathGroup)]), nil
}
