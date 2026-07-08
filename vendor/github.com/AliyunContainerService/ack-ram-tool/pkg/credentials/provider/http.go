package provider

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type httpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type httpError struct {
	code    int
	message string
	data    []byte
}

type commonHttpClient struct {
	client httpClient
	logger Logger
}

func newCommonHttpClient(transport http.RoundTripper, timeout time.Duration) *commonHttpClient {
	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}
	return &commonHttpClient{client: client}
}

func (c *commonHttpClient) send(ctx context.Context, method, url string, header http.Header, body io.Reader) (string, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return "", fmt.Errorf("can not init request with url %s: %w", url, err)
	}
	req = req.WithContext(ctx)
	req.Header.Set("User-Agent", UserAgent)
	for k, items := range header {
		for _, v := range items {
			req.Header.Add(k, v)
		}
	}

	if debugMode {
		for _, item := range genDebugReqMessages(req) {
			c.getLogger().Debug(item)
		}
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request %s failed: %w", url, err)
	}
	defer resp.Body.Close()

	if debugMode {
		for _, item := range genDebugRespMessages(resp) {
			c.getLogger().Debug(item)
		}
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body failed when request %s: %w", url, err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", &httpError{
			code: resp.StatusCode,
			message: fmt.Sprintf("status code %d is not 200 when request %s: %s",
				resp.StatusCode, url, strings.ReplaceAll(string(data), "\n", " ")),
			data: data,
		}
	}

	return string(data), nil
}

func (c *commonHttpClient) getLogger() Logger {
	if c.logger != nil {
		return c.logger
	}
	return defaultLog
}

func (e httpError) Error() string {
	return e.message
}
