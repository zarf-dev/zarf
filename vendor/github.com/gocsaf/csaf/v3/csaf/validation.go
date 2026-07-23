// This file is Free Software under the Apache-2.0 License
// without warranty, see README.md and LICENSES/Apache-2.0.txt for details.
//
// SPDX-License-Identifier: Apache-2.0
//
// SPDX-FileCopyrightText: 2021 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
// Software-Engineering: 2021 Intevation GmbH <https://intevation.de>

package csaf

import (
	"bytes"
	"crypto/tls"
	_ "embed" // Used for embedding.
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed schema/csaf_json_schema.json
var csafSchema []byte

//go:embed schema/cvss-v2.0.json
var cvss20 []byte

//go:embed schema/cvss-v3.0.json
var cvss30 []byte

//go:embed schema/cvss-v3.1.json
var cvss31 []byte

//go:embed schema/provider_json_schema.json
var providerSchema []byte

//go:embed schema/aggregator_json_schema.json
var aggregatorSchema []byte

//go:embed schema/ROLIE_feed_json_schema.json
var rolieSchema []byte

type compiledSchema struct {
	url      string
	once     sync.Once
	err      error
	compiled *jsonschema.Schema
}

const (
	csafSchemaURL       = "https://docs.oasis-open.org/csaf/csaf/v2.0/csaf_json_schema.json"
	providerSchemaURL   = "https://docs.oasis-open.org/csaf/csaf/v2.0/provider_json_schema.json"
	aggregatorSchemaURL = "https://docs.oasis-open.org/csaf/csaf/v2.0/aggregator_json_schema.json"
	cvss20SchemaURL     = "https://www.first.org/cvss/cvss-v2.0.json"
	cvss30SchemaURL     = "https://www.first.org/cvss/cvss-v3.0.json"
	cvss31SchemaURL     = "https://www.first.org/cvss/cvss-v3.1.json"
	rolieSchemaURL      = "https://raw.githubusercontent.com/tschmidtb51/csaf/ROLIE-schema/csaf_2.0/json_schema/ROLIE_feed_json_schema.json"
)

var (
	compiledCSAFSchema       = compiledSchema{url: csafSchemaURL}
	compiledProviderSchema   = compiledSchema{url: providerSchemaURL}
	compiledAggregatorSchema = compiledSchema{url: aggregatorSchemaURL}
	compiledRolieSchema      = compiledSchema{url: rolieSchemaURL}
)

type schemaLoader http.Client

func (l *schemaLoader) loadHTTPURL(url string) (any, error) {
	client := (*http.Client)(l)
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned status code %d", url, resp.StatusCode)
	}

	return jsonschema.UnmarshalJSON(resp.Body)
}

// Load loads the schema from the specified url.
func (l *schemaLoader) Load(url string) (any, error) {
	loader := func(data []byte) (any, error) {
		return jsonschema.UnmarshalJSON(bytes.NewReader(data))
	}
	switch url {
	case csafSchemaURL:
		return loader(csafSchema)
	case cvss20SchemaURL:
		return loader(cvss20)
	case cvss30SchemaURL:
		return loader(cvss30)
	case cvss31SchemaURL:
		return loader(cvss31)
	case providerSchemaURL:
		return loader(providerSchema)
	case aggregatorSchemaURL:
		return loader(aggregatorSchema)
	case rolieSchemaURL:
		return loader(rolieSchema)
	default:
		// Fallback to http loader
		return l.loadHTTPURL(url)
	}
}

func newSchemaLoader(insecure bool) *schemaLoader {
	httpLoader := schemaLoader(http.Client{
		Timeout: 15 * time.Second,
	})
	if insecure {
		httpLoader.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}
	return &httpLoader
}

func (cs *compiledSchema) compile() {
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	c.UseLoader(newSchemaLoader(false))
	cs.compiled, cs.err = c.Compile(cs.url)
}

func (cs *compiledSchema) validate(doc any) ([]string, error) {
	cs.once.Do(cs.compile)

	if cs.err != nil {
		return nil, cs.err
	}

	err := cs.compiled.Validate(doc)
	if err == nil {
		return nil, nil
	}

	var valErr *jsonschema.ValidationError
	ok := errors.As(err, &valErr)
	if !ok {
		return nil, err
	}

	basic := valErr.BasicOutput()
	if basic.Valid {
		return nil, nil
	}

	errs := basic.Errors

	sort.Slice(errs, func(i, j int) bool {
		pi := errs[i].InstanceLocation
		pj := errs[j].InstanceLocation
		if strings.HasPrefix(pj, pi) {
			return true
		}
		if strings.HasPrefix(pi, pj) {
			return false
		}
		if pi != pj {
			return pi < pj
		}
		return errs[i].Error.String() < errs[j].Error.String()
	})

	res := make([]string, 0, len(errs))

	for i := range errs {
		e := &errs[i]
		if e.Error == nil {
			continue
		}
		loc := e.InstanceLocation
		if loc == "" {
			loc = e.AbsoluteKeywordLocation
		}
		res = append(res, loc+": "+e.Error.String())
	}

	return res, nil
}

// ValidateCSAF validates the document doc against the JSON schema
// of CSAF.
func ValidateCSAF(doc any) ([]string, error) {
	return compiledCSAFSchema.validate(doc)
}

// ValidateProviderMetadata validates the document doc against the JSON schema
// of provider metadata.
func ValidateProviderMetadata(doc any) ([]string, error) {
	return compiledProviderSchema.validate(doc)
}

// ValidateAggregator validates the document doc against the JSON schema
// of aggregator.
func ValidateAggregator(doc any) ([]string, error) {
	return compiledAggregatorSchema.validate(doc)
}

// ValidateROLIE validates the ROLIE feed against the JSON schema
// of ROLIE
func ValidateROLIE(doc any) ([]string, error) {
	return compiledRolieSchema.validate(doc)
}
