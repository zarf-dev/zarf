// Copyright 2015-2017 Piprate Limited
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ld

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"time"

	"github.com/pquerna/cachecontrol"
)

const (
	// An HTTP Accept header that prefers JSONLD.
	acceptHeader = "application/ld+json, application/json;q=0.9, application/javascript;q=0.5, text/javascript;q=0.5, text/plain;q=0.2, */*;q=0.1"

	ApplicationJSONLDType = "application/ld+json"

	// JSON-LD link header rel
	linkHeaderRel = "http://www.w3.org/ns/json-ld#context"
)

// RemoteDocument is a document retrieved from a remote source.
type RemoteDocument struct {
	DocumentURL string
	Document    interface{}
	ContextURL  string
}

// DocumentLoader knows how to load remote documents.
type DocumentLoader interface {
	LoadDocument(u string) (*RemoteDocument, error)
}

// DefaultDocumentLoader is a standard implementation of DocumentLoader
// which can retrieve documents via HTTP.
type DefaultDocumentLoader struct {
	httpClient *http.Client
}

// NewDefaultDocumentLoader creates a new instance of DefaultDocumentLoader
func NewDefaultDocumentLoader(httpClient *http.Client) *DefaultDocumentLoader {
	rval := &DefaultDocumentLoader{httpClient: httpClient}

	if rval.httpClient == nil {
		rval.httpClient = http.DefaultClient
	}
	return rval
}

// DocumentFromReader returns a document containing the contents of the JSON resource,
// streamed from the given Reader.
func DocumentFromReader(r io.Reader) (interface{}, error) {
	var document interface{}
	dec := json.NewDecoder(r)

	// If dec.UseNumber() were invoked here, all numbers would be decoded as json.Number.
	// json-gold supports both the default and json.Number options.

	if err := dec.Decode(&document); err != nil {
		return nil, NewJsonLdError(LoadingDocumentFailed, err)
	}
	return document, nil
}

// LoadDocument returns a RemoteDocument containing the contents of the JSON resource
// from the given URL.
func (dl *DefaultDocumentLoader) LoadDocument(u string) (*RemoteDocument, error) {
	parsedURL, err := url.Parse(u)
	if err != nil {
		return nil, NewJsonLdError(LoadingDocumentFailed, fmt.Sprintf("error parsing URL: %s", u))
	}

	remoteDoc := &RemoteDocument{}

	protocol := parsedURL.Scheme
	if protocol != "http" && protocol != "https" {
		// Can't use the HTTP client for those!
		remoteDoc.DocumentURL = u
		var file *os.File
		file, err = os.Open(u)
		if err != nil {
			return nil, NewJsonLdError(LoadingDocumentFailed, err)
		}
		defer file.Close()

		remoteDoc.Document, err = DocumentFromReader(file)
		if err != nil {
			return nil, NewJsonLdError(LoadingDocumentFailed, err)
		}
	} else {

		req, err := http.NewRequest("GET", u, http.NoBody)
		if err != nil {
			return nil, NewJsonLdError(LoadingDocumentFailed, err)
		}
		// We prefer application/ld+json, but fallback to application/json
		// or whatever is available
		req.Header.Add("Accept", acceptHeader)

		res, err := dl.httpClient.Do(req)
		if err != nil {
			return nil, NewJsonLdError(LoadingDocumentFailed, err)
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			return nil, NewJsonLdError(LoadingDocumentFailed,
				fmt.Sprintf("Bad response status code: %d", res.StatusCode))
		}

		remoteDoc.DocumentURL = res.Request.URL.String()

		contentType := res.Header.Get("Content-Type")
		linkHeader := res.Header.Get("Link")

		if len(linkHeader) > 0 {
			parsedLinkHeader := ParseLinkHeader(linkHeader)
			contextLink := parsedLinkHeader[linkHeaderRel]
			if contextLink != nil && contentType != ApplicationJSONLDType &&
				(contentType == "application/json" || rApplicationJSON.MatchString(contentType)) {

				if len(contextLink) > 1 {
					return nil, NewJsonLdError(MultipleContextLinkHeaders, nil)
				} else if len(contextLink) == 1 {
					remoteDoc.ContextURL = contextLink[0]["target"]
				}
			}

			// If content-type is not application/ld+json, nor any other +json
			// and a link with rel=alternate and type='application/ld+json' is found,
			// use that instead
			alternateLink := parsedLinkHeader["alternate"]
			if len(alternateLink) > 0 &&
				alternateLink[0]["type"] == ApplicationJSONLDType &&
				!rApplicationJSON.MatchString(contentType) {

				finalURL := Resolve(u, alternateLink[0]["target"])
				return dl.LoadDocument(finalURL)
			}
		}

		remoteDoc.Document, err = DocumentFromReader(res.Body)
		if err != nil {
			return nil, NewJsonLdError(LoadingDocumentFailed, err)
		}
	}
	return remoteDoc, nil
}

var rSplitOnComma = regexp.MustCompile("(?:<[^>]*?>|\"[^\"]*?\"|[^,])+")
var rLinkHeader = regexp.MustCompile(`\s*<([^>]*?)>\s*(?:;\s*(.*))?`)
var rApplicationJSON = regexp.MustCompile(`^application/(\w*\+)?json$`)
var rParams = regexp.MustCompile("(.*?)=(?:(?:\"([^\"]*?)\")|([^\"]*?))\\s*(?:(?:;\\s*)|$)")

// ParseLinkHeader parses a link header. The results will be keyed by the value of "rel".
//
//	Link: <http://json-ld.org/contexts/person.jsonld>; \
//	  rel="http://www.w3.org/ns/json-ld#context"; type="application/ld+json"
//
//	Parses as: {
//	  'http://www.w3.org/ns/json-ld#context': {
//	    target: http://json-ld.org/contexts/person.jsonld,
//	    rel:    http://www.w3.org/ns/json-ld#context
//	  }
//	}
//
// If there is more than one "rel" with the same IRI, then entries in the
// resulting map for that "rel" will be lists.
func ParseLinkHeader(header string) map[string][]map[string]string {

	rval := make(map[string][]map[string]string)

	// split on unbracketed/unquoted commas
	entries := rSplitOnComma.FindAllString(header, -1)
	if len(entries) == 0 {
		return rval
	}

	for _, entry := range entries {
		if !rLinkHeader.MatchString(entry) {
			continue
		}
		match := rLinkHeader.FindStringSubmatch(entry)

		result := map[string]string{
			"target": match[1],
		}
		params := match[2]
		matches := rParams.FindAllStringSubmatch(params, -1)
		for _, match := range matches {
			if match[2] == "" {
				result[match[1]] = match[3]
			} else {
				result[match[1]] = match[2]
			}
		}
		rel := result["rel"]
		relVal, hasRel := rval[rel]
		if hasRel {
			rval[rel] = append(relVal, result)
		} else {
			rval[rel] = []map[string]string{result}
		}
	}
	return rval
}

// CachingDocumentLoader is an overlay on top of DocumentLoader instance
// which allows caching documents as soon as they get retrieved
// from the underlying loader. You may also preload it with documents -
// this is useful for testing.
type CachingDocumentLoader struct {
	nextLoader DocumentLoader
	cache      map[string]*RemoteDocument
}

// NewCachingDocumentLoader creates a new instance of CachingDocumentLoader.
func NewCachingDocumentLoader(nextLoader DocumentLoader) *CachingDocumentLoader {
	rval := &CachingDocumentLoader{
		nextLoader: nextLoader,
		cache:      make(map[string]*RemoteDocument),
	}

	return rval
}

// LoadDocument returns a RemoteDocument containing the contents of the JSON resource
// from the given URL.
func (cdl *CachingDocumentLoader) LoadDocument(u string) (*RemoteDocument, error) {
	if doc, cached := cdl.cache[u]; cached {
		return doc, nil
	} else {
		doc, err := cdl.nextLoader.LoadDocument(u)
		if err != nil {
			return nil, err
		}
		cdl.cache[u] = doc
		return doc, nil
	}
}

// AddDocument populates the cache with the given document (doc) for the provided URL (u).
func (cdl *CachingDocumentLoader) AddDocument(u string, doc interface{}) {
	cdl.cache[u] = &RemoteDocument{DocumentURL: u, Document: doc, ContextURL: ""}
}

// PreloadWithMapping populates the cache with a number of documents which may be loaded
// from location different from the original URL (most importantly, from local files).
//
// Example:
//
//	l.PreloadWithMapping(map[string]string{
//	    "http://www.example.com/context.json": "/home/me/cache/example_com_context.json",
//	})
func (cdl *CachingDocumentLoader) PreloadWithMapping(urlMap map[string]string) error {
	for srcURL, mappedURL := range urlMap {
		doc, err := cdl.nextLoader.LoadDocument(mappedURL)
		if err != nil {
			return err
		}
		cdl.cache[srcURL] = doc
	}
	return nil
}

type cachedRemoteDocument struct {
	remoteDocument *RemoteDocument
	expireTime     time.Time
	neverExpires   bool
}

// RFC7324CachingDocumentLoader respects RFC7324 caching headers in order to
// cache effectively
type RFC7324CachingDocumentLoader struct {
	httpClient *http.Client
	cache      map[string]*cachedRemoteDocument
}

// NewRFC7324CachingDocumentLoader creates a new RFC7324CachingDocumentLoader
func NewRFC7324CachingDocumentLoader(httpClient *http.Client) *RFC7324CachingDocumentLoader {
	rval := &RFC7324CachingDocumentLoader{
		httpClient: httpClient,
		cache:      make(map[string]*cachedRemoteDocument),
	}

	if httpClient == nil {
		rval.httpClient = http.DefaultClient
	}

	return rval
}

// LoadDocument returns a RemoteDocument containing the contents of the JSON resource
// from the given URL.
func (rcdl *RFC7324CachingDocumentLoader) LoadDocument(u string) (*RemoteDocument, error) {
	entry, ok := rcdl.cache[u]
	now := time.Now()

	// First we check if we hit in the cache, and the cache entry is valid
	// We need to check if expireTime >= now, so we negate the comparison below
	if ok && (entry.neverExpires || entry.expireTime.After(now)) {
		return entry.remoteDocument, nil
	}

	parsedURL, err := url.Parse(u)
	if err != nil {
		return nil, NewJsonLdError(LoadingDocumentFailed, fmt.Sprintf("error parsing URL: %s", u))
	}

	remoteDoc := &RemoteDocument{}

	// We use neverExpires, shouldCache, and expireTime at the end of this method
	// to create an object to store in the cache. Set them to sane default values now
	neverExpires := false
	shouldCache := false
	expireTime := time.Now()

	protocol := parsedURL.Scheme
	if protocol != "http" && protocol != "https" {
		// Can't use the HTTP client for those!
		remoteDoc.DocumentURL = u
		var file *os.File
		file, err = os.Open(u)
		if err != nil {
			return nil, NewJsonLdError(LoadingDocumentFailed, err)
		}
		defer file.Close()
		remoteDoc.Document, err = DocumentFromReader(file)
		if err != nil {
			return nil, NewJsonLdError(LoadingDocumentFailed, err)
		}
		neverExpires = true
		shouldCache = true
	} else {

		req, err := http.NewRequest("GET", u, http.NoBody)
		if err != nil {
			return nil, NewJsonLdError(LoadingDocumentFailed, err)
		}
		// We prefer application/ld+json, but fallback to application/json
		// or whatever is available
		req.Header.Add("Accept", acceptHeader)

		res, err := rcdl.httpClient.Do(req)
		if err != nil {
			return nil, NewJsonLdError(LoadingDocumentFailed, err)
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			return nil, NewJsonLdError(LoadingDocumentFailed,
				fmt.Sprintf("Bad response status code: %d", res.StatusCode))
		}

		remoteDoc.DocumentURL = res.Request.URL.String()

		contentType := res.Header.Get("Content-Type")
		linkHeader := res.Header.Get("Link")

		if len(linkHeader) > 0 {
			parsedLinkHeader := ParseLinkHeader(linkHeader)
			contextLink := parsedLinkHeader[linkHeaderRel]
			if contextLink != nil && contentType != ApplicationJSONLDType {
				if len(contextLink) > 1 {
					return nil, NewJsonLdError(MultipleContextLinkHeaders, nil)
				} else if len(contextLink) == 1 {
					remoteDoc.ContextURL = contextLink[0]["target"]
				}
			}

			// If content-type is not application/ld+json, nor any other +json
			// and a link with rel=alternate and type='application/ld+json' is found,
			// use that instead
			alternateLink := parsedLinkHeader["alternate"]
			if len(alternateLink) > 0 &&
				alternateLink[0]["type"] == ApplicationJSONLDType &&
				!rApplicationJSON.MatchString(contentType) {

				finalURL := Resolve(u, alternateLink[0]["target"])
				remoteDoc, err = rcdl.LoadDocument(finalURL)
				if err != nil {
					return nil, NewJsonLdError(LoadingDocumentFailed, err)
				}
			}
		}

		reasons, resExpireTime, err := cachecontrol.CachableResponse(req, res, cachecontrol.Options{})
		// If there are no errors parsing cache headers and there are no reasons not to cache, then we cache
		if err == nil && len(reasons) == 0 {
			shouldCache = true
			expireTime = resExpireTime
		}

		if remoteDoc.Document == nil {
			remoteDoc.Document, err = DocumentFromReader(res.Body)
			if err != nil {
				return nil, NewJsonLdError(LoadingDocumentFailed, err)
			}
		}
	}

	// If we went down a branch that marked shouldCache true then lets add the cache entry into
	// the cache
	if shouldCache {
		cacheEntry := &cachedRemoteDocument{
			remoteDocument: remoteDoc,
			expireTime:     expireTime,
			neverExpires:   neverExpires,
		}
		rcdl.cache[u] = cacheEntry
	}

	return remoteDoc, nil
}
