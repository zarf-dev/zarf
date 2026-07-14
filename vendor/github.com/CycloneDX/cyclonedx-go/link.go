// This file is part of CycloneDX Go
//
// Licensed under the Apache License, Version 2.0 (the “License”);
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an “AS IS” BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0
// Copyright (c) OWASP Foundation. All Rights Reserved.

package cyclonedx

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// BOMLink provides the ability to create references to other
// BOMs and specific components, services or vulnerabilities within them.
//
// See also:
// - https://cyclonedx.org/capabilities/bomlink/
// - https://www.iana.org/assignments/urn-formal/cdx
type BOMLink struct {
	serialNumber string // Serial number of the linked BOM
	version      int    // Version of the linked BOM
	reference    string // Reference of the linked element
}

// NewBOMLink creates a new link to a BOM with a given serial number and version.
// The serial number MUST conform to RFC-4122. The version MUST NOT be zero or negative.
//
// By providing a non-nil element, a deep link to that element is created.
// Linkable elements include components, services and vulnerabilities.
// When an element is provided, it MUST have a bom reference.
func NewBOMLink(serial string, version int, elem interface{}) (link BOMLink, err error) {
	if !serialNumberRegex.MatchString(serial) {
		err = fmt.Errorf("invalid serial number")
		return
	}
	if version < 1 {
		err = fmt.Errorf("invalid version: must not be negative or zero")
		return
	}

	ref := ""
	if elem != nil {
		switch elem := elem.(type) {
		case Component:
			ref = elem.BOMRef
		case *Component:
			ref = elem.BOMRef
		case Service:
			ref = elem.BOMRef
		case *Service:
			ref = elem.BOMRef
		case Vulnerability:
			ref = elem.BOMRef
		case *Vulnerability:
			ref = elem.BOMRef
		default:
			err = fmt.Errorf("element of type %T is not linkable", elem)
			return
		}
		if ref == "" {
			err = fmt.Errorf("the provided element does not have a bom reference")
			return
		}
	}

	return BOMLink{
		serialNumber: serial,
		version:      version,
		reference:    ref,
	}, nil
}

// SerialNumber returns the serial number of the linked BOM.
func (b BOMLink) SerialNumber() string {
	return b.serialNumber
}

// Version returns the version of the linked BOM.
func (b BOMLink) Version() int {
	return b.version
}

// Reference returns the reference of the element within the linked BOM.
func (b BOMLink) Reference() string {
	return b.reference
}

// String returns the string representation of the link.
func (b BOMLink) String() string {
	if b.reference == "" {
		return fmt.Sprintf("urn:cdx:%s/%d", strings.TrimPrefix(b.serialNumber, "urn:uuid:"), b.version)
	}

	return fmt.Sprintf("urn:cdx:%s/%d#%s", strings.TrimPrefix(b.serialNumber, "urn:uuid:"), b.version, url.QueryEscape(b.reference))
}

var bomLinkRegex = regexp.MustCompile(`^urn:cdx:(?P<serial>[\da-f]{8}-[\da-f]{4}-[\da-f]{4}-[\da-f]{4}-[\da-f]{12})/(?P<version>[1-9]\d*)(?:#(?P<ref>[\da-zA-Z\-._~%!$&'()*+,;=:@/?]+))?$`)

// IsBOMLink checks whether a given string is a valid BOM link.
func IsBOMLink(s string) bool {
	return bomLinkRegex.MatchString(s)
}

// ParseBOMLink parses a string into a BOMLink.
func ParseBOMLink(s string) (link BOMLink, err error) {
	matches := bomLinkRegex.FindStringSubmatch(s)
	if matches == nil {
		err = fmt.Errorf("invalid bom link")
		return
	}

	serial := "urn:uuid:" + matches[1]
	version, err := strconv.Atoi(matches[2])
	if err != nil {
		err = fmt.Errorf("failed to parse version: %w", err)
		return
	}

	ref := ""
	if len(matches) == 4 {
		ref, err = url.QueryUnescape(matches[3])
		if err != nil {
			err = fmt.Errorf("failed to unescape reference: %w", err)
			return
		}
	}

	return BOMLink{
		serialNumber: serial,
		version:      version,
		reference:    ref,
	}, nil
}
