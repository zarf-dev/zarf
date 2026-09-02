package internal

import (
	"fmt"
	"net/url"
	"reflect"
	"strings"

	"github.com/google/uuid"
)

const (
	DefaultSpdxDocumentIDPrefix   = "https://spdx.org/spdxdocs/"
	DefaultSpdxNamespaceSeparator = "#"
	DefaultSpdxNamespace          = "SPDXRef"
)

func NewDocumentID(documentName string) string {
	// SPDX 2 suggested document namespace, which is effectively the SpdxDocument ID is:
	// https://[CreatorWebsite]/[pathToSpdx]/[DocumentName]-[UUID]
	// with the note: if the creator does not own their own website, a default SPDX CreatorWebsite and PathToSpdx can be used spdx.org/spdxdocs.
	// If a user does not provide their own base documentID, we will continue to use this, in absence of other guidance rather than
	// a significantly more expensive UUID generation per element
	if documentName == "" {
		documentName = uuid.New().String()
	} else {
		documentName = url.PathEscape(documentName)
	}
	return DefaultSpdxDocumentIDPrefix + documentName
}

func PrefixedIdGenerator(iriPrefix string, prefixes map[string]string) IdGeneratorFunc {
	nextID := map[reflect.Type]int{}

	applyPrefix := func(s string) string {
		for uri, prefix := range prefixes {
			if strings.HasPrefix(s, uri) {
				return prefix + ":" + s[len(uri):]
			}
		}
		return s
	}

	return func(id string, v reflect.Value) string {
		t := v.Type()
		if t.Kind() == reflect.Pointer {
			t = t.Elem()
		}
		if id == "" {
			num := nextID[t] + 1
			nextID[t] = num
			id = fmt.Sprintf("%s-%d", t.Name(), num)
		}
		if IsURI(id) {
			return applyPrefix(id)
		}
		if blankNodeAllowed(v.Type()) {
			return "_:" + id
		}
		if id == "SpdxDocument-1" {
			id = "DOCUMENT"
		}
		return iriPrefix + ":" + id
	}
}

func uuidGenerator() IdGeneratorFunc {
	return func(existingId string, value reflect.Value) string {
		u := uuid.New()
		return fmt.Sprintf("urn:uuid:%v", u.String())
	}
}
