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
	"net/url"
	"regexp"
	"strings"
)

// JsonLdUrl represents a URL split into individual components
// for easy manipulation.
// TODO: This implementation was taken from Java. Do we really need it in Go?
type JsonLdUrl struct { //nolint:stylecheck
	Href      string
	Protocol  string
	Host      string
	Auth      string
	User      string
	Password  string
	Hostname  string
	Port      string
	Relative  string
	Path      string
	Directory string
	File      string
	Query     string
	Hash      string

	// things not populated by the regex
	Pathname       string
	NormalizedPath string
	Authority      string
}

var parser = regexp.MustCompile(`^(?:([^:/?#]+):)?(?://((?:(([^:@]*)(?::([^:@]*))?)?@)?([^:/?#]*)(?::(\d*))?))?((((?:[^?#/]*/)*)([^?#]*))(?:\?([^#]*))?(?:#(.*))?)`)

// ParseURL parses a string URL into JsonLdUrl struct.
func ParseURL(urlStr string) *JsonLdUrl {
	rval := JsonLdUrl{Href: urlStr}

	if parser.MatchString(urlStr) {
		matches := parser.FindStringSubmatch(urlStr)
		if matches[1] != "" {
			rval.Protocol = matches[1]
		}
		if matches[2] != "" {
			rval.Host = matches[2]
		}
		if matches[3] != "" {
			rval.Auth = matches[3]
		}
		if matches[4] != "" {
			rval.User = matches[4]
		}
		if matches[5] != "" {
			rval.Password = matches[5]
		}
		if matches[6] != "" {
			rval.Hostname = matches[6]
		}
		if matches[7] != "" {
			rval.Port = matches[7]
		}
		if matches[8] != "" {
			rval.Relative = matches[8]
		}
		if matches[9] != "" {
			rval.Path = matches[9]
		}
		if matches[10] != "" {
			rval.Directory = matches[10]
		}
		if matches[11] != "" {
			rval.File = matches[11]
		}
		if matches[12] != "" {
			rval.Query = matches[12]
		}
		if matches[13] != "" {
			rval.Hash = matches[13]
		}

		// normalize to node.js API
		if rval.Host != "" && rval.Path == "" {
			rval.Path = "/"
		}

		rval.Pathname = rval.Path
		parseAuthority(&rval)
		rval.NormalizedPath = removeDotSegments(rval.Pathname, rval.Authority != "")
		if rval.Query != "" {
			rval.Path += "?" + rval.Query
		}
		if rval.Protocol != "" {
			rval.Protocol += ":"
		}
		if rval.Hash != "" {
			rval.Hash = "#" + rval.Hash
		}
	}

	return &rval
}

// removeDotSegments removes dot segments from a JsonLdUrl path.
func removeDotSegments(path string, hasAuthority bool) string {
	var rval []byte
	if strings.HasPrefix(path, "/") {
		rval = append(rval, '/')
	}

	// RFC 3986 5.2.4 (reworked)
	input := strings.Split(path, "/")
	var output = make([]string, 0)
	for i := 0; i < len(input); i++ {
		if input[i] == "." || (input[i] == "" && len(input)-i > 1) {
			continue
		}
		if input[i] == ".." {
			if hasAuthority || (len(output) > 0 && output[len(output)-1] != "..") {
				if len(output) > 0 {
					output = output[:len(output)-1]
				}
			} else {
				output = append(output, "..")
			}
			continue
		}
		output = append(output, input[i])
	}

	if len(output) > 0 {
		rval = append(rval, output[0]...)
		for i := 1; i < len(output); i++ {
			rval = append(rval, '/')
			rval = append(rval, output[i]...)
		}
	}
	return string(rval)
}

// RemoveBase removes base URL from the given IRI.
func RemoveBase(baseobj interface{}, iri string) string {
	if baseobj == nil {
		return iri
	}

	var base *JsonLdUrl
	if baseStr, isString := baseobj.(string); isString {
		base = ParseURL(baseStr)
	} else {
		base = baseobj.(*JsonLdUrl)
	}

	// establish base root
	root := ""
	if base.Href != "" {
		root += base.Protocol + "//" + base.Authority
	} else if !strings.HasPrefix(iri, "//") {
		// support network-path reference with empty base
		root += "//"
	}

	// IRI not relative to base
	if strings.Index(iri, root) != 0 {
		return iri
	}

	// remove root from IRI and parse remainder
	rel := ParseURL(iri[len(root):])

	// remove path segments that match
	baseSegments := strings.Split(base.NormalizedPath, "/")
	iriSegments := strings.Split(rel.NormalizedPath, "/")

	last := 1
	if len(rel.Hash) > 0 || len(rel.Query) > 0 {
		last = 0
	}

	for len(baseSegments) > 0 && len(iriSegments) > last && baseSegments[0] == iriSegments[0] {
		baseSegments = baseSegments[1:]
		iriSegments = iriSegments[1:]
	}

	// use '../' for each non-matching base segment
	rval := ""

	if len(baseSegments) > 0 {
		// don't count the last segment if it isn't a path (doesn't end in
		// '/')
		// don't count empty first segment, it means base began with '/'
		if !strings.HasSuffix(base.NormalizedPath, "/") || baseSegments[0] == "" {
			baseSegments = baseSegments[0 : len(baseSegments)-1]
		}
		for i := 0; i < len(baseSegments); i++ {
			rval += "../"
		}
	}

	// prepend remaining segments
	if len(iriSegments) > 0 {
		rval += iriSegments[0]
	}
	for i := 1; i < len(iriSegments); i++ {
		rval += "/" + iriSegments[i]
	}

	// add query and hash
	if rel.Query != "" {
		rval += "?" + rel.Query
	}
	if rel.Hash != "" {
		rval += rel.Hash
	}

	if rval == "" {
		rval = "./"
	}

	return rval
}

// Resolve the given path against the given base URI.
// Returns a full URI.
func Resolve(baseURI string, pathToResolve string) string {
	if baseURI == "" {
		return pathToResolve
	}
	if pathToResolve == "" || strings.TrimSpace(pathToResolve) == "" {
		return baseURI
	}

	uri, _ := url.Parse(baseURI)
	// query string parsing
	if strings.HasPrefix(pathToResolve, "?") {
		// drop fragment from uri if it has one
		uri.Fragment = ""
		uri.RawQuery = pathToResolve[1:]
		return uri.String()
	}

	pathToResolveURL, _ := url.Parse(pathToResolve)
	uri = uri.ResolveReference(pathToResolveURL)
	// java doesn't discard unnecessary dot segments
	if uri.Path != "" {
		uri.Path = removeDotSegments(uri.Path, true)
	}
	return uri.String()
}

// parseAuthority parses the authority for the pre-parsed given JsonLdUrl.
func parseAuthority(parsed *JsonLdUrl) {
	// parse authority for unparsed relative network-path reference
	if !strings.Contains(parsed.Href, ":") && strings.HasPrefix(parsed.Href, "//") && parsed.Host == "" {
		// must parse authority from pathname
		parsed.Pathname = parsed.Pathname[2:]
		idx := strings.Index(parsed.Pathname, "/")
		if idx == -1 {
			parsed.Authority = parsed.Pathname
			parsed.Pathname = ""
		} else {
			parsed.Authority = parsed.Pathname[0:idx]
			parsed.Pathname = parsed.Pathname[idx:]
		}
	} else {
		// construct authority
		parsed.Authority = parsed.Host
		if parsed.Auth != "" {
			parsed.Authority = parsed.Auth + "@" + parsed.Authority
		}
	}
}
