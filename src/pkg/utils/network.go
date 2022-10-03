package utils

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
)

func IsUrl(source string) bool {
	parsedUrl, err := url.Parse(source)
	return err == nil && parsedUrl.Scheme != "" && parsedUrl.Host != ""
}

// DoesHostnamesMatch returns a boolean indicating if the hostname of two different URLs are the same.
func DoesHostnamesMatch(url1 string, url2 string) (bool, error) {
	parsedURL1, err := url.Parse(url1)
	if err != nil {
		return false, err
	}
	parsedURL2, err := url.Parse(url2)
	if err != nil {
		return false, err
	}

	return parsedURL1.Hostname() == parsedURL2.Hostname(), nil
}

func Fetch(url string) (io.ReadCloser, error) {
	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	// Check server response
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Bad HTTP status: %s", resp.Status)
	}

	return resp.Body, nil
}
