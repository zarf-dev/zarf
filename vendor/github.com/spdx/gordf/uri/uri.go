// Implementation of URIRef required for nodes in the rdf graph.

package uri

import (
	"fmt"
	"net/url"
	"strings"
)

type URIRef struct {
	/**
	 * A URI Reference is formed of one or two components:
	 * 		Base: base URL / URI. Can optionally end in # char
	 * 		Fragment: relative component of url     [optional]
	 * A valid uri is:
	 *     base#fragment or base#
	 * For example:
	 * 		https://www.w3.org/TR/skos-reference/#L1302 is a valid URIRef with
	 * 		    Base = https://www.w3.org/TR/skos-reference/
	 * 		    Fragment = L1302
	 */
	uri string
}

// constructor for URIRef
func NewURIRef(uri string) (uriref URIRef, err error) {
	/**
	 * Usage and equivalence:
	 * 		base := "https://www.w3.org/TR/skos-reference/"
	 * 		fragment: "L1302"
	 * 		uriref := NewURIRef(base, fragment)
	 * 		uriref -> "https://www.w3.org/TR/skos-reference/#L1302"
	 */

	// validating the input uri
	_, err = url.ParseRequestURI(uri)
	if err != nil {
		return uriref, fmt.Errorf("Malformed URI: %v", err)
	}

	// adding a # to the end if it doesn't end in # already.
	if !strings.HasSuffix(uri, "#") {
		uri += "#"
	}

	// validate uri after addition of # at the end
	return URIRef{uri}, err
}

// join the fragment to the uri of current object
func (uriref *URIRef) AddFragment(frag string) (retURI URIRef) {
	if strings.HasPrefix(frag, "#") {
		frag = frag[1:]
	}

	// validating the relative uri
	_, err := url.ParseRequestURI(uriref.uri + frag)
	if err != nil {
		return
	}

	// relative uri is fine, return a new object of uriref.
	return URIRef{uriref.uri + frag}
}

// returns string representation of the uriref
func (uriref *URIRef) String() string {
	return uriref.uri
}
