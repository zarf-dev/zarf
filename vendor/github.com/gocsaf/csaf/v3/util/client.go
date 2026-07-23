// This file is Free Software under the Apache-2.0 License
// without warranty, see README.md and LICENSES/Apache-2.0.txt for details.
//
// SPDX-License-Identifier: Apache-2.0
//
// SPDX-FileCopyrightText: 2022 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
// Software-Engineering: 2022 Intevation GmbH <https://intevation.de>

package util //revive:disable-line:var-naming

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/time/rate"
)

// Client is an interface to abstract http.Client.
type Client interface {
	Do(req *http.Request) (*http.Response, error)
	Get(url string) (*http.Response, error)
	Head(url string) (*http.Response, error)
	Post(url, contentType string, body io.Reader) (*http.Response, error)
	PostForm(url string, data url.Values) (*http.Response, error)
}

// LoggingClient is a client that logs called URLs.
type LoggingClient struct {
	Client
	Log func(method, url string)
}

// LimitingClient is a Client implementing rate throttling.
type LimitingClient struct {
	Client
	Limiter *rate.Limiter
}

// HeaderClient adds extra HTTP header fields to requests.
type HeaderClient struct {
	Client
	Header http.Header
}

// Do implements the respective method of the [Client] interface.
func (hc *HeaderClient) Do(req *http.Request) (*http.Response, error) {
	// Maybe this overly careful but this minimizes
	// potential side effects in the caller.
	orig := req.Header
	defer func() { req.Header = orig }()

	// Work on a copy.
	req.Header = req.Header.Clone()

	for key, values := range hc.Header {
		for _, v := range values {
			req.Header.Add(key, v)
		}
	}

	// Use default user agent if none is set
	if userAgent := hc.Header.Get("User-Agent"); userAgent == "" {
		req.Header.Add("User-Agent", "csaf_distribution/"+SemVersion)
	}
	return hc.Client.Do(req)
}

// Get implements the respective method of the [Client] interface.
func (hc *HeaderClient) Get(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return hc.Do(req)
}

// Head implements the respective method of the [Client] interface.
func (hc *HeaderClient) Head(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return nil, err
	}
	return hc.Do(req)
}

// Post implements the respective method of the [Client] interface.
func (hc *HeaderClient) Post(url, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	return hc.Do(req)
}

// PostForm implements the respective method of the [Client] interface.
func (hc *HeaderClient) PostForm(url string, data url.Values) (*http.Response, error) {
	return hc.Post(
		url, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
}

// log logs to a callback if given.
func (lc *LoggingClient) log(method, url string) {
	if lc.Log != nil {
		lc.Log(method, url)
	} else {
		log.Printf("[%s]: %s\n", method, url)
	}
}

// Do implements the respective method of the Client interface.
func (lc *LoggingClient) Do(req *http.Request) (*http.Response, error) {
	lc.log("DO", req.URL.String())
	return lc.Client.Do(req)
}

// Get implements the respective method of the Client interface.
func (lc *LoggingClient) Get(url string) (*http.Response, error) {
	lc.log("GET", url)
	return lc.Client.Get(url)
}

// Head implements the respective method of the Client interface.
func (lc *LoggingClient) Head(url string) (*http.Response, error) {
	lc.log("HEAD", url)
	return lc.Client.Head(url)
}

// Post implements the respective method of the Client interface.
func (lc *LoggingClient) Post(url, contentType string, body io.Reader) (*http.Response, error) {
	lc.log("POST", url)
	return lc.Client.Post(url, contentType, body)
}

// PostForm implements the respective method of the Client interface.
func (lc *LoggingClient) PostForm(url string, data url.Values) (*http.Response, error) {
	lc.log("POST FORM", url)
	return lc.Client.PostForm(url, data)
}

// Do implements the respective method of the Client interface.
func (lc *LimitingClient) Do(req *http.Request) (*http.Response, error) {
	lc.Limiter.Wait(context.Background())
	return lc.Client.Do(req)
}

// Get implements the respective method of the Client interface.
func (lc *LimitingClient) Get(url string) (*http.Response, error) {
	lc.Limiter.Wait(context.Background())
	return lc.Client.Get(url)
}

// Head implements the respective method of the Client interface.
func (lc *LimitingClient) Head(url string) (*http.Response, error) {
	lc.Limiter.Wait(context.Background())
	return lc.Client.Head(url)
}

// Post implements the respective method of the Client interface.
func (lc *LimitingClient) Post(url, contentType string, body io.Reader) (*http.Response, error) {
	lc.Limiter.Wait(context.Background())
	return lc.Client.Post(url, contentType, body)
}

// PostForm implements the respective method of the Client interface.
func (lc *LimitingClient) PostForm(url string, data url.Values) (*http.Response, error) {
	lc.Limiter.Wait(context.Background())
	return lc.Client.PostForm(url, data)
}
