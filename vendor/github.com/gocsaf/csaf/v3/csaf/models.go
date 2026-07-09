// This file is Free Software under the Apache-2.0 License
// without warranty, see README.md and LICENSES/Apache-2.0.txt for details.
//
// SPDX-License-Identifier: Apache-2.0
//
// SPDX-FileCopyrightText: 2022 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
// Software-Engineering: 2022 Intevation GmbH <https://intevation.de>

package csaf

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/gocsaf/csaf/v3/internal/misc"
	"github.com/gocsaf/csaf/v3/util"
)

// TLPLabel is the traffic light policy of the CSAF.
type TLPLabel string

const (
	// TLPLabelUnlabeled is the 'UNLABELED' policy.
	TLPLabelUnlabeled = "UNLABELED"
	// TLPLabelWhite is the 'WHITE' policy.
	TLPLabelWhite = "WHITE"
	// TLPLabelGreen is the 'GREEN' policy.
	TLPLabelGreen = "GREEN"
	// TLPLabelAmber is the 'AMBER' policy.
	TLPLabelAmber = "AMBER"
	// TLPLabelRed is the 'RED' policy.
	TLPLabelRed = "RED"
)

var tlpLabelPattern = alternativesUnmarshal(
	TLPLabelUnlabeled,
	TLPLabelWhite,
	TLPLabelGreen,
	TLPLabelAmber,
	TLPLabelRed,
)

// JSONURL is an URL to JSON document.
type JSONURL string

var jsonURLPattern = patternUnmarshal(`\.json$`)

// Feed is CSAF feed.
type Feed struct {
	Summary  string    `json:"summary"`
	TLPLabel *TLPLabel `json:"tlp_label"` // required
	URL      *JSONURL  `json:"url"`       // required
}

// ROLIE is the ROLIE extension of the CSAF feed.
type ROLIE struct {
	Categories []JSONURL `json:"categories,omitempty"`
	Feeds      []Feed    `json:"feeds"` // required
	Services   []JSONURL `json:"services,omitempty"`
}

// Distribution is a distribution of a CSAF feed.
type Distribution struct {
	DirectoryURL string `json:"directory_url,omitempty"`
	Rolie        *ROLIE `json:"rolie,omitempty"`
}

// TimeStamp represents a time stamp in a CSAF feed.
type TimeStamp time.Time

// Fingerprint is the fingerprint of a OpenPGP key used to sign
// the CSAF documents.
type Fingerprint string

var fingerprintPattern = patternUnmarshal(`^[0-9a-fA-F]{40,}$`)

// PGPKey is location and the fingerprint of the key
// used to sign the CSAF documents.
type PGPKey struct {
	Fingerprint Fingerprint `json:"fingerprint,omitempty"`
	URL         *string     `json:"url"` // required
}

// Category is the category of the CSAF feed.
type Category string

const (
	// CSAFCategoryCoordinator is the "coordinator" category.
	CSAFCategoryCoordinator Category = "coordinator"
	// CSAFCategoryDiscoverer is the "discoverer" category.
	CSAFCategoryDiscoverer Category = "discoverer"
	// CSAFCategoryOther is the "other" category.
	CSAFCategoryOther Category = "other"
	// CSAFCategoryTranslator is the "translator" category.
	CSAFCategoryTranslator Category = "translator"
	// CSAFCategoryUser is the "user" category.
	CSAFCategoryUser Category = "user"
	// CSAFCategoryVendor is the "vendor" category.
	CSAFCategoryVendor Category = "vendor"
)

var csafCategoryPattern = alternativesUnmarshal(
	string(CSAFCategoryCoordinator),
	string(CSAFCategoryDiscoverer),
	string(CSAFCategoryOther),
	string(CSAFCategoryTranslator),
	string(CSAFCategoryUser),
	string(CSAFCategoryVendor))

// Publisher is the publisher of the feed.
type Publisher struct {
	Category         *Category `json:"category" toml:"category"`   // required
	Name             *string   `json:"name" toml:"name"`           // required
	Namespace        *string   `json:"namespace" toml:"namespace"` // required
	ContactDetails   string    `json:"contact_details,omitempty" toml:"contact_details"`
	IssuingAuthority string    `json:"issuing_authority,omitempty" toml:"issuing_authority"`
}

// MetadataVersion is the metadata version of the feed.
type MetadataVersion string

// MetadataVersion20 is the current version of the schema.
const MetadataVersion20 MetadataVersion = "2.0"

var metadataVersionPattern = alternativesUnmarshal(string(MetadataVersion20))

// MetadataRole is the role of the feed.
type MetadataRole string

const (
	// MetadataRolePublisher is the "csaf_publisher" role.
	MetadataRolePublisher MetadataRole = "csaf_publisher"
	// MetadataRoleProvider is the "csaf_provider" role.
	MetadataRoleProvider MetadataRole = "csaf_provider"
	// MetadataRoleTrustedProvider is the "csaf_trusted_provider" role.
	MetadataRoleTrustedProvider MetadataRole = "csaf_trusted_provider"
)

var metadataRolePattern = alternativesUnmarshal(
	string(MetadataRolePublisher),
	string(MetadataRoleProvider),
	string(MetadataRoleTrustedProvider))

// ProviderURL is the URL of the provider document.
type ProviderURL string

var providerURLPattern = patternUnmarshal(`/provider-metadata\.json$`)

// ProviderMetadata contains the metadata of the provider.
type ProviderMetadata struct {
	CanonicalURL            *ProviderURL     `json:"canonical_url"` // required
	Distributions           []Distribution   `json:"distributions,omitempty"`
	LastUpdated             *TimeStamp       `json:"last_updated"` // required
	ListOnCSAFAggregators   *bool            `json:"list_on_CSAF_aggregators"`
	MetadataVersion         *MetadataVersion `json:"metadata_version"`           // required
	MirrorOnCSAFAggregators *bool            `json:"mirror_on_CSAF_aggregators"` // required
	PGPKeys                 []PGPKey         `json:"public_openpgp_keys,omitempty"`
	Publisher               *Publisher       `json:"publisher,omitempty"` // required
	Role                    *MetadataRole    `json:"role"`                // required
}

// AggregatorCategory is the category of the aggregator.
type AggregatorCategory string

const (
	// AggregatorAggregator represents the "aggregator" type of aggregators.
	AggregatorAggregator AggregatorCategory = "aggregator"
	// AggregatorLister represents the "listers" type of aggregators.
	AggregatorLister AggregatorCategory = "lister"
)

var aggregatorCategoryPattern = alternativesUnmarshal(
	string(AggregatorAggregator),
	string(AggregatorLister),
)

// AggregatorVersion is the version of the aggregator.
type AggregatorVersion string

const (
	// AggregatorVersion20 is version 2.0 of the aggregator.
	AggregatorVersion20 AggregatorVersion = "2.0"
)

var aggregatorVersionPattern = alternativesUnmarshal(
	string(AggregatorVersion20),
)

// AggregatorInfo reflects the 'aggregator' object in the aggregator.
type AggregatorInfo struct {
	Category         *AggregatorCategory `json:"category,omitempty" toml:"category"` // required
	Name             string              `json:"name" toml:"name"`                   // required
	ContactDetails   string              `json:"contact_details,omitempty" toml:"contact_details"`
	IssuingAuthority string              `json:"issuing_authority,omitempty" toml:"issuing_authority"`
	Namespace        string              `json:"namespace" toml:"namespace"` // required
}

// AggregatorURL is the URL of the aggregator document.
type AggregatorURL string

var aggregatorURLPattern = patternUnmarshal(`/aggregator\.json$`)

// AggregatorCSAFProviderMetadata reflects 'csaf_providers.metadata' in an aggregator.
type AggregatorCSAFProviderMetadata struct {
	LastUpdated *TimeStamp    `json:"last_updated,omitempty"` // required
	Publisher   *Publisher    `json:"publisher,omitempty"`    // required
	Role        *MetadataRole `json:"role,omitempty"`
	URL         *ProviderURL  `json:"url,omitempty"` // required
}

// AggregatorCSAFProvider reflects one 'csaf_trusted_provider' in an aggregator.
type AggregatorCSAFProvider struct {
	Metadata *AggregatorCSAFProviderMetadata `json:"metadata,omitempty"` // required
	Mirrors  []ProviderURL                   `json:"mirrors,omitempty"`  // required
}

// AggregatorCSAFPublisher reflects one publisher in an aggregator.
type AggregatorCSAFPublisher struct {
	Metadata       *AggregatorCSAFProviderMetadata `json:"metadata,omitempty"`        // required
	Mirrors        []ProviderURL                   `json:"mirrors,omitempty"`         // required
	UpdateInterval string                          `json:"update_interval,omitempty"` // required
}

// Aggregator is the CSAF Aggregator.
type Aggregator struct {
	Aggregator     *AggregatorInfo            `json:"aggregator,omitempty"`         // required
	Version        *AggregatorVersion         `json:"aggregator_version,omitempty"` // required
	CanonicalURL   *AggregatorURL             `json:"canonical_url,omitempty"`      // required
	CSAFProviders  []*AggregatorCSAFProvider  `json:"csaf_providers,omitempty"`     // required
	CSAFPublishers []*AggregatorCSAFPublisher `json:"csaf_publishers,omitempty"`
	LastUpdated    *TimeStamp                 `json:"last_updated,omitempty"` // required
}

// Validate validates the current state of the AggregatorCategory.
func (ac *AggregatorCategory) Validate() error {
	if ac == nil {
		return errors.New("aggregator.aggregator.category is mandatory")
	}
	return nil
}

// Validate validates the current state of the AggregatorVersion.
func (av *AggregatorVersion) Validate() error {
	if av == nil {
		return errors.New("aggregator.aggregator_version is mandatory")
	}
	return nil
}

// Validate validates the current state of the AggregatorURL.
func (au *AggregatorURL) Validate() error {
	if au == nil {
		return errors.New("aggregator.aggregator_url is mandatory")
	}
	return nil
}

// Validate validates the current state of the AggregatorInfo.
func (ai *AggregatorInfo) Validate() error {
	if err := ai.Category.Validate(); err != nil {
		return err
	}
	if ai.Name == "" {
		return errors.New("aggregator.aggregator.name is mandatory")
	}
	if ai.Namespace == "" {
		return errors.New("aggregator.aggregator.namespace is mandatory")
	}
	return nil
}

// Validate validates the current state of the AggregatorCSAFProviderMetadata.
func (acpm *AggregatorCSAFProviderMetadata) Validate() error {
	if acpm == nil {
		return errors.New("aggregator.csaf_providers[].metadata is mandatory")
	}
	if acpm.LastUpdated == nil {
		return errors.New("aggregator.csaf_providers[].metadata.last_updated is mandatory")
	}
	if acpm.Publisher == nil {
		return errors.New("aggregator.csaf_providers[].metadata.publisher is mandatory")
	}
	if err := acpm.Publisher.Validate(); err != nil {
		return err
	}
	if acpm.URL == nil {
		return errors.New("aggregator.csaf_providers[].metadata.url is mandatory")
	}
	return nil
}

// Validate validates the current state of the AggregatorCSAFProvider.
func (acp *AggregatorCSAFProvider) Validate() error {
	if acp == nil {
		return errors.New("aggregator.csaf_providers[] not allowed to be nil")
	}
	return acp.Metadata.Validate()
}

// Validate validates the current state of the Aggregator.
func (a *Aggregator) Validate() error {
	if err := a.Aggregator.Validate(); err != nil {
		return err
	}
	if err := a.Version.Validate(); err != nil {
		return err
	}
	if err := a.CanonicalURL.Validate(); err != nil {
		return err
	}
	for _, provider := range a.CSAFProviders {
		if err := provider.Validate(); err != nil {
			return err
		}
	}
	if a.LastUpdated == nil {
		return errors.New("aggregator.LastUpdate == nil")
	}
	return nil
}

// UnmarshalText implements the encoding.TextUnmarshaller interface.
func (mdv *MetadataVersion) UnmarshalText(data []byte) error {
	s, err := metadataVersionPattern(data)
	if err == nil {
		*mdv = MetadataVersion(s)
	}
	return err
}

// UnmarshalText implements the encoding.TextUnmarshaller interface.
func (mdr *MetadataRole) UnmarshalText(data []byte) error {
	s, err := metadataRolePattern(data)
	if err == nil {
		*mdr = MetadataRole(s)
	}
	return err
}

// UnmarshalText implements the encoding.TextUnmarshaller interface.
func (ac *AggregatorCategory) UnmarshalText(data []byte) error {
	s, err := aggregatorCategoryPattern(data)
	if err == nil {
		*ac = AggregatorCategory(s)
	}
	return err
}

// UnmarshalText implements the encoding.TextUnmarshaller interface.
func (av *AggregatorVersion) UnmarshalText(data []byte) error {
	s, err := aggregatorVersionPattern(data)
	if err == nil {
		*av = AggregatorVersion(s)
	}
	return err
}

// UnmarshalText implements the encoding.TextUnmarshaller interface.
func (au *AggregatorURL) UnmarshalText(data []byte) error {
	s, err := aggregatorURLPattern(data)
	if err == nil {
		*au = AggregatorURL(s)
	}
	return err
}

func patternUnmarshal(pattern string) func([]byte) (string, error) {
	r := regexp.MustCompile(pattern)
	return func(data []byte) (string, error) {
		s := string(data)
		if !r.MatchString(s) {
			return "", fmt.Errorf("%s does not match %v", s, r)
		}
		return s, nil
	}
}

func alternativesUnmarshal(alternatives ...string) func([]byte) (string, error) {
	return func(data []byte) (string, error) {
		s := string(data)
		for _, alt := range alternatives {
			if alt == s {
				return s, nil
			}
		}
		return "", fmt.Errorf("%s not in [%s]", s, strings.Join(alternatives, "|"))
	}
}

// UnmarshalText implements the encoding.TextUnmarshaller interface.
func (tl *TLPLabel) UnmarshalText(data []byte) error {
	s, err := tlpLabelPattern(data)
	if err == nil {
		*tl = TLPLabel(s)
	}
	return err
}

// UnmarshalText implements the encoding.TextUnmarshaller interface.
func (ju *JSONURL) UnmarshalText(data []byte) error {
	s, err := jsonURLPattern(data)
	if err == nil {
		*ju = JSONURL(s)
	}
	return err
}

// UnmarshalText implements the encoding.TextUnmarshaller interface.
func (pu *ProviderURL) UnmarshalText(data []byte) error {
	s, err := providerURLPattern(data)
	if err == nil {
		*pu = ProviderURL(s)
	}
	return err
}

// UnmarshalText implements the encoding.TextUnmarshaller interface.
func (cc *Category) UnmarshalText(data []byte) error {
	s, err := csafCategoryPattern(data)
	if err == nil {
		*cc = Category(s)
	}
	return err
}

// UnmarshalText implements the encoding.TextUnmarshaller interface.
func (fp *Fingerprint) UnmarshalText(data []byte) error {
	s, err := fingerprintPattern(data)
	if err == nil {
		*fp = Fingerprint(s)
	}
	return err
}

// UnmarshalText implements the encoding.TextUnmarshaller interface.
func (ts *TimeStamp) UnmarshalText(data []byte) error {
	t, err := time.Parse(time.RFC3339, string(data))
	if err != nil {
		return err
	}
	*ts = TimeStamp(t)
	return nil
}

// MarshalText implements the encoding.TextMarshaller interface.
func (ts TimeStamp) MarshalText() ([]byte, error) {
	return []byte(time.Time(ts).Format(time.RFC3339)), nil
}

// Defaults fills the correct default values into the provider metadata.
func (pmd *ProviderMetadata) Defaults() {
	if pmd.Role == nil {
		role := MetadataRoleTrustedProvider
		pmd.Role = &role
	}
	if pmd.ListOnCSAFAggregators == nil {
		t := true
		pmd.ListOnCSAFAggregators = &t
	}
	if pmd.MirrorOnCSAFAggregators == nil {
		t := true
		pmd.MirrorOnCSAFAggregators = &t
	}
	if pmd.MetadataVersion == nil {
		mdv := MetadataVersion20
		pmd.MetadataVersion = &mdv
	}
}

// AddDirectoryDistribution adds a directory based distribution
// with a given url to the provider metadata.
func (pmd *ProviderMetadata) AddDirectoryDistribution(url string) {
	// Avoid duplicates.
	for i := range pmd.Distributions {
		if pmd.Distributions[i].DirectoryURL == url {
			return
		}
	}
	pmd.Distributions = append(pmd.Distributions, Distribution{DirectoryURL: url})
}

// Validate checks if the feed is valid.
// Returns an error if the validation fails otherwise nil.
func (f *Feed) Validate() error {
	switch {
	case f.TLPLabel == nil:
		return errors.New("feed[].tlp_label is mandatory")
	case f.URL == nil:
		return errors.New("feed[].url is mandatory")
	}
	return nil
}

// Validate checks if the ROLIE extension is valid.
// Returns an error if the validation fails otherwise nil.
func (r *ROLIE) Validate() error {
	if len(r.Feeds) < 1 {
		return errors.New("ROLIE needs at least one feed")
	}
	for i := range r.Feeds {
		if err := r.Feeds[i].Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Validate checks if the publisher is valid.
// Returns an error if the validation fails otherwise nil.
func (p *Publisher) Validate() error {
	switch {
	case p == nil:
		return errors.New("publisher is mandatory")
	case p.Category == nil:
		return errors.New("publisher.category is mandatory")
	case p.Name == nil:
		return errors.New("publisher.name is mandatory")
	case p.Namespace == nil:
		return errors.New("publisher.namespace is mandatory")
	}
	return nil
}

func strPtrEquals(a, b *string) bool {
	switch {
	case a == nil:
		return b == nil
	case b == nil:
		return false
	default:
		return *a == *b
	}
}

// Equals checks if the publisher is equal to other componentwise.
func (p *Publisher) Equals(o *Publisher) bool {
	switch {
	case p == nil:
		return o == nil
	case o == nil:
		return false
	default:
		return strPtrEquals((*string)(p.Category), (*string)(o.Category)) &&
			strPtrEquals(p.Name, o.Name) &&
			strPtrEquals(p.Namespace, o.Namespace) &&
			p.ContactDetails == o.ContactDetails &&
			p.IssuingAuthority == o.IssuingAuthority
	}
}

// Validate checks if the PGPKey is valid.
// Returns an error if the validation fails otherwise nil.
func (pk *PGPKey) Validate() error {
	if pk.URL == nil {
		return errors.New("pgp_key[].url is mandatory")
	}
	return nil
}

// Validate checks if the distribution is valid.
// Returns an error if the validation fails otherwise nil.
func (d *Distribution) Validate() error {
	if d.Rolie != nil {
		if err := d.Rolie.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Validate checks if the provider metadata is valid.
// Returns an error if the validation fails otherwise nil.
func (pmd *ProviderMetadata) Validate() error {
	switch {
	case pmd.CanonicalURL == nil:
		return errors.New("canonical_url is mandatory")
	case pmd.LastUpdated == nil:
		return errors.New("last_updated is mandatory")
	case pmd.MetadataVersion == nil:
		return errors.New("metadata_version is mandatory")
	}

	if err := pmd.Publisher.Validate(); err != nil {
		return err
	}

	for i := range pmd.PGPKeys {
		if err := pmd.PGPKeys[i].Validate(); err != nil {
			return err
		}
	}

	for i := range pmd.Distributions {
		if err := pmd.Distributions[i].Validate(); err != nil {
			return err
		}
	}

	return nil
}

// SetLastUpdated updates the last updated timestamp of the feed.
func (pmd *ProviderMetadata) SetLastUpdated(t time.Time) {
	ts := TimeStamp(t.UTC())
	pmd.LastUpdated = &ts
}

// SetPGP sets the fingerprint and URL of the OpenPGP key
// of the feed. If the feed already has a key with
// given fingerprint the URL updated.
// If there is no such key it is append to the list of keys.
func (pmd *ProviderMetadata) SetPGP(fingerprint, url string) {
	for i := range pmd.PGPKeys {
		if strings.EqualFold(string(pmd.PGPKeys[i].Fingerprint), fingerprint) {
			pmd.PGPKeys[i].URL = &url
			return
		}
	}
	pmd.PGPKeys = append(pmd.PGPKeys, PGPKey{
		Fingerprint: Fingerprint(fingerprint),
		URL:         &url,
	})
}

// NewProviderMetadata creates a new provider with the given URL.
// Valid default values are set and the feed is considered to
// be updated recently.
func NewProviderMetadata(canonicalURL string) *ProviderMetadata {
	pm := new(ProviderMetadata)
	pm.Defaults()
	pm.SetLastUpdated(time.Now())
	cu := ProviderURL(canonicalURL)
	pm.CanonicalURL = &cu
	return pm
}

// NewProviderMetadataDomain creates a new provider with the given URL
// and tlps feeds.
func NewProviderMetadataDomain(domain string, tlps []TLPLabel) *ProviderMetadata {
	return NewProviderMetadataPrefix(domain+"/.well-known/csaf", tlps)
}

// NewProviderMetadataPrefix creates a new provider with a given prefix
// and tlps feeds.
func NewProviderMetadataPrefix(prefix string, tlps []TLPLabel) *ProviderMetadata {

	pm := NewProviderMetadata(
		prefix + "/provider-metadata.json")

	if len(tlps) == 0 {
		return pm
	}

	// Register feeds.

	feeds := make([]Feed, len(tlps))

	for i, t := range tlps {
		lt := strings.ToLower(string(t))
		feed := "csaf-feed-tlp-" + lt + ".json"
		url := JSONURL(prefix + "/" + lt + "/" + feed)

		t := t
		feeds[i] = Feed{
			Summary:  "TLP:" + string(t) + " advisories",
			TLPLabel: &t,
			URL:      &url,
		}
	}

	pm.Distributions = []Distribution{{
		Rolie: &ROLIE{
			Feeds: feeds,
		},
	}}

	return pm
}

// WriteTo saves a metadata provider to a writer.
func (pmd *ProviderMetadata) WriteTo(w io.Writer) (int64, error) {
	nw := util.NWriter{Writer: w, N: 0}
	enc := json.NewEncoder(&nw)
	enc.SetIndent("", "  ")
	err := enc.Encode(pmd)
	return nw.N, err
}

// LoadProviderMetadata loads a metadata provider from a reader.
func LoadProviderMetadata(r io.Reader) (*ProviderMetadata, error) {

	var pmd ProviderMetadata
	if err := misc.StrictJSONParse(r, &pmd); err != nil {
		return nil, err
	}

	if err := pmd.Validate(); err != nil {
		return nil, err
	}

	// Set defaults.
	pmd.Defaults()

	return &pmd, nil
}

// WriteTo saves an AggregatorURL to a writer.
func (a *Aggregator) WriteTo(w io.Writer) (int64, error) {
	nw := util.NWriter{Writer: w, N: 0}
	enc := json.NewEncoder(&nw)
	enc.SetIndent("", "  ")
	err := enc.Encode(a)
	return nw.N, err
}
