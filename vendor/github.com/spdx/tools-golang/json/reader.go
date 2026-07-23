// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later

package json

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"

	"github.com/spdx/tools-golang/convert"
	"github.com/spdx/tools-golang/spdx"
	"github.com/spdx/tools-golang/spdx/common"
	"github.com/spdx/tools-golang/spdx/v2/v2_1"
	"github.com/spdx/tools-golang/spdx/v2/v2_2"
	"github.com/spdx/tools-golang/spdx/v2/v2_3"
	"github.com/spdx/tools-golang/spdx/v3/v3_0"
)

// Read takes an io.Reader and returns a fully-parsed current model SPDX Document
// or an error if any error is encountered.
func Read(content io.Reader) (*spdx.Document, error) {
	doc := spdx.Document{}
	err := ReadInto(content, &doc)
	return &doc, err
}

// ReadInto takes an io.Reader, reads in the SPDX document at the version provided
// and converts to the doc version
func ReadInto(content io.Reader, doc common.AnyDocument) error {
	if !convert.IsPtr(doc) {
		return fmt.Errorf("doc to read into must be a pointer")
	}

	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(content)
	if err != nil {
		return err
	}

	var data interface{}
	err = json.Unmarshal(buf.Bytes(), &data)
	if err != nil {
		return err
	}

	val, ok := data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("not a valid SPDX JSON document")
	}

	version, _ := val["spdxVersion"].(string)
	if version == "" {
		version, _ = val["@context"].(string)
		if version != "" {
			extract := regexp.MustCompile(`https://spdx.org/rdf/(\d+(?:\.\d+)+)/spdx-context\.jsonld`)
			matches := extract.FindStringSubmatch(version)
			if len(matches) == 2 {
				version = matches[1]
			}
		}
	}

	if version == "" {
		return fmt.Errorf("JSON document does not contain spdxVersion field")
	}

	switch version {
	case v2_1.Version:
		var doc v2_1.Document
		err = json.Unmarshal(buf.Bytes(), &doc)
		if err != nil {
			return err
		}
		data = doc
	case v2_2.Version:
		var doc v2_2.Document
		err = json.Unmarshal(buf.Bytes(), &doc)
		if err != nil {
			return err
		}
		data = doc
	case v2_3.Version:
		var doc v2_3.Document
		err = json.Unmarshal(buf.Bytes(), &doc)
		if err != nil {
			return err
		}
		data = doc
	case "3.0.0":
		fallthrough
	case v3_0.Version:
		// support older 3.0.x versions
		contents := buf.Bytes()
		if version != v3_0.Version {
			contents = bytes.Replace(contents, []byte(version), []byte(fmt.Sprintf("https://spdx.org/rdf/%s/spdx-context.jsonld", v3_0.Version)), 1)
		}
		var in v3_0.Document
		err = json.Unmarshal(contents, &in)
		if err != nil {
			return err
		}
		data = in
	default:
		return fmt.Errorf("unsupported SDPX version: %s", version)
	}

	return convert.Document(data, doc)
}
