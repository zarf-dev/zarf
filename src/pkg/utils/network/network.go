package network

import (
	"net/url"

	"github.com/defenseunicorns/zarf/src/pkg/transform"
)

// DoHostnamesMatch returns a boolean indicating if the hostname of two different URLs are the same.
func DoHostnamesMatch(url1 string, url2 string) (bool, error) {
	parsedURL1, err := url.Parse(url1)
	if err != nil {
		return false, fmt.Errorf("unable to parse the url (%s): %w", url1, err)
	}
	parsedURL2, err := url.Parse(url2)
	if err != nil {
		return false, fmt.Errorf("unable to parse the url (%s): %w", url2, err)
	}

	return parsedURL1.Hostname() == parsedURL2.Hostname(), nil
}
