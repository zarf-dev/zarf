package network

import (
	"net/url"

	"github.com/defenseunicorns/zarf/src/pkg/transform"
)

// DoHostnamesMatch returns a boolean indicating if the hostname of two different URLs are the same.
func DoHostnamesMatch(logger transform.Log, url1 string, url2 string) (bool, error) {
	parsedURL1, err := url.Parse(url1)
	if err != nil {
		logger("unable to parse the url (%s)", url1)
		return false, err
	}
	parsedURL2, err := url.Parse(url2)
	if err != nil {
		logger("unable to parse the url (%s)", url2)
		return false, err
	}

	return parsedURL1.Hostname() == parsedURL2.Hostname(), nil
}
