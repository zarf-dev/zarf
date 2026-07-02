// This file is Free Software under the Apache-2.0 License
// without warranty, see README.md and LICENSES/Apache-2.0.txt for details.
//
// SPDX-License-Identifier: Apache-2.0
//
// SPDX-FileCopyrightText: 2023 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
// Software-Engineering: 2023 Intevation GmbH <https://intevation.de>

package csaf

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/gocsaf/csaf/v3/internal/misc"
)

// Acknowledgement reflects the 'acknowledgement' object in the list of acknowledgements.
// It must at least have one property.
type Acknowledgement struct {
	Names        []*string `json:"names,omitempty"`
	Organization *string   `json:"organization,omitempty"`
	Summary      *string   `json:"summary,omitempty"`
	URLs         []*string `json:"urls,omitempty"`
}

// Acknowledgements is a list of Acknowledgement elements.
type Acknowledgements []*Acknowledgement

// BranchCategory is the category of a branch.
type BranchCategory string

const (
	// CSAFBranchCategoryArchitecture is the "architecture" category.
	CSAFBranchCategoryArchitecture BranchCategory = "architecture"
	// CSAFBranchCategoryHostName is the "host_name" category.
	CSAFBranchCategoryHostName BranchCategory = "host_name"
	// CSAFBranchCategoryLanguage is the "language" category.
	CSAFBranchCategoryLanguage BranchCategory = "language"
	// CSAFBranchCategoryLegacy is the "legacy" category.
	CSAFBranchCategoryLegacy BranchCategory = "legacy"
	// CSAFBranchCategoryPatchLevel is the "patch_level" category.
	CSAFBranchCategoryPatchLevel BranchCategory = "patch_level"
	// CSAFBranchCategoryProductFamily is the "product_family" category.
	CSAFBranchCategoryProductFamily BranchCategory = "product_family"
	// CSAFBranchCategoryProductName is the "product_name" category.
	CSAFBranchCategoryProductName BranchCategory = "product_name"
	// CSAFBranchCategoryProductVersion is the "product_version" category.
	CSAFBranchCategoryProductVersion BranchCategory = "product_version"
	// CSAFBranchCategoryProductVersionRange is the "product_version_range" category.
	CSAFBranchCategoryProductVersionRange BranchCategory = "product_version_range"
	// CSAFBranchCategoryServicePack is the "service_pack" category.
	CSAFBranchCategoryServicePack BranchCategory = "service_pack"
	// CSAFBranchCategorySpecification is the "specification" category.
	CSAFBranchCategorySpecification BranchCategory = "specification"
	// CSAFBranchCategoryVendor is the "vendor" category.
	CSAFBranchCategoryVendor BranchCategory = "vendor"
)

var csafBranchCategoryPattern = alternativesUnmarshal(
	string(CSAFBranchCategoryArchitecture),
	string(CSAFBranchCategoryHostName),
	string(CSAFBranchCategoryLanguage),
	string(CSAFBranchCategoryLegacy),
	string(CSAFBranchCategoryPatchLevel),
	string(CSAFBranchCategoryProductFamily),
	string(CSAFBranchCategoryProductName),
	string(CSAFBranchCategoryProductVersion),
	string(CSAFBranchCategoryProductVersionRange),
	string(CSAFBranchCategoryServicePack),
	string(CSAFBranchCategorySpecification),
	string(CSAFBranchCategoryVendor))

// ProductID is a reference token for product instances. There is no predefined or
// required format for it as long as it uniquely identifies a product in the context
// of the current document.
type ProductID string

// Products is a list of one or more unique ProductID elements.
type Products []*ProductID

// FileHashValue represents the value of a hash.
type FileHashValue string

var fileHashValuePattern = patternUnmarshal(`^[0-9a-fA-F]{32,}$`)

// FileHash is checksum hash.
// Values for 'algorithm' are derived from the currently supported digests OpenSSL. Leading dashes were removed.
type FileHash struct {
	Algorithm *string        `json:"algorithm"` // required, default: sha256
	Value     *FileHashValue `json:"value"`     // required
}

// Hashes is a list of hashes.
type Hashes struct {
	FileHashes []*FileHash `json:"file_hashes"` // required
	FileName   *string     `json:"filename"`    // required
}

// CPE represents a Common Platform Enumeration in an advisory.
type CPE string

var cpePattern = patternUnmarshal("^(cpe:2\\.3:[aho\\*\\-](:(((\\?*|\\*?)([a-zA-Z0-9\\-\\._]|(\\\\[\\\\\\*\\?!\"#\\$%&'\\(\\)\\+,/:;<=>@\\[\\]\\^`\\{\\|\\}~]))+(\\?*|\\*?))|[\\*\\-])){5}(:(([a-zA-Z]{2,3}(-([a-zA-Z]{2}|[0-9]{3}))?)|[\\*\\-]))(:(((\\?*|\\*?)([a-zA-Z0-9\\-\\._]|(\\\\[\\\\\\*\\?!\"#\\$%&'\\(\\)\\+,/:;<=>@\\[\\]\\^`\\{\\|\\}~]))+(\\?*|\\*?))|[\\*\\-])){4})|([c][pP][eE]:/[AHOaho]?(:[A-Za-z0-9\\._\\-~%]*){0,6})$")

// PURL represents a package URL in an advisory.
type PURL string

var pURLPattern = patternUnmarshal("^pkg:[A-Za-z\\.\\-\\+][A-Za-z0-9\\.\\-\\+]*/.+")

// XGenericURI represents an identifier for a product.
type XGenericURI struct {
	Namespace *string `json:"namespace"` //  required
	URI       *string `json:"uri"`       //  required
}

// XGenericURIs is a list of XGenericURI.
type XGenericURIs []*XGenericURI

// ProductIdentificationHelper bundles product identifier information.
// Supported formats for SBOMs are SPDX, CycloneDX, and SWID
type ProductIdentificationHelper struct {
	CPE           *CPE         `json:"cpe,omitempty"`
	Hashes        *Hashes      `json:"hashes,omitempty"`
	ModelNumbers  []*string    `json:"model_numbers,omitempty"` // unique elements
	PURL          *PURL        `json:"purl,omitempty"`
	SBOMURLs      []*string    `json:"sbom_urls,omitempty"`
	SerialNumbers []*string    `json:"serial_numbers,omitempty"` // unique elements
	SKUs          []*string    `json:"skus,omitempty"`
	XGenericURIs  XGenericURIs `json:"x_generic_uris,omitempty"`
}

// FullProductName is the full name of a product.
type FullProductName struct {
	Name                        *string                      `json:"name"`       // required
	ProductID                   *ProductID                   `json:"product_id"` // required
	ProductIdentificationHelper *ProductIdentificationHelper `json:"product_identification_helper,omitempty"`
}

// FullProductNames is a list of FullProductName.
type FullProductNames []*FullProductName

// Branch reflects the 'branch' object in the list of branches.
// It may contain either the property Branches OR Product.
// If the category is 'product_version' the name MUST NOT contain
// version ranges of any kind.
// If the category is 'product_version_range' the name MUST contain
// version ranges.
type Branch struct {
	Branches Branches         `json:"branches,omitempty"`
	Category *BranchCategory  `json:"category"` // required
	Name     *string          `json:"name"`     // required
	Product  *FullProductName `json:"product,omitempty"`
}

// NoteCategory is the category of a note.
type NoteCategory string

const (
	// CSAFNoteCategoryDescription is the "description" category.
	CSAFNoteCategoryDescription NoteCategory = "description"
	// CSAFNoteCategoryDetails is the "details" category.
	CSAFNoteCategoryDetails NoteCategory = "details"
	// CSAFNoteCategoryFaq is the "faq" category.
	CSAFNoteCategoryFaq NoteCategory = "faq"
	// CSAFNoteCategoryGeneral is the "general" category.
	CSAFNoteCategoryGeneral NoteCategory = "general"
	// CSAFNoteCategoryLegalDisclaimer is the "legal_disclaimer" category.
	CSAFNoteCategoryLegalDisclaimer NoteCategory = "legal_disclaimer"
	// CSAFNoteCategoryOther is the "other" category.
	CSAFNoteCategoryOther NoteCategory = "other"
	// CSAFNoteCategorySummary is the "summary" category.
	CSAFNoteCategorySummary NoteCategory = "summary"
)

var csafNoteCategoryPattern = alternativesUnmarshal(
	string(CSAFNoteCategoryDescription),
	string(CSAFNoteCategoryDetails),
	string(CSAFNoteCategoryFaq),
	string(CSAFNoteCategoryGeneral),
	string(CSAFNoteCategoryLegalDisclaimer),
	string(CSAFNoteCategoryOther),
	string(CSAFNoteCategorySummary))

// Note reflects the 'Note' object of an advisory.
type Note struct {
	Audience     *string       `json:"audience,omitempty"`
	NoteCategory *NoteCategory `json:"category"` // required
	Text         *string       `json:"text"`     // required
	Title        *string       `json:"title,omitempty"`
}

// ReferenceCategory is the category of a note.
type ReferenceCategory string

const (
	// CSAFReferenceCategoryExternal is the "external" category.
	CSAFReferenceCategoryExternal ReferenceCategory = "external"
	// CSAFReferenceCategorySelf is the "self" category.
	CSAFReferenceCategorySelf ReferenceCategory = "self"
)

var csafReferenceCategoryPattern = alternativesUnmarshal(
	string(CSAFReferenceCategoryExternal),
	string(CSAFReferenceCategorySelf))

// Reference holding any reference to conferences, papers, advisories, and other
// resources that are related and considered related to either a surrounding part of
// or the entire document and to be of value to the document consumer.
type Reference struct {
	ReferenceCategory *string `json:"category"` // optional, default: external
	Summary           *string `json:"summary"`  // required
	URL               *string `json:"url"`      // required
}

// AggregateSeverity stands for the urgency with which the vulnerabilities of an advisory
// (not a specific one) should be addressed.
type AggregateSeverity struct {
	Namespace *string `json:"namespace,omitempty"`
	Text      *string `json:"text"` // required
}

// DocumentCategory represents a category of a document.
type DocumentCategory string

var documentCategoryPattern = patternUnmarshal("^[^\\s\\-_\\.](.*[^\\s\\-_\\.])?$")

// Version is the version of a document.
type Version string

// CSAFVersion20 is the current version of CSAF.
const CSAFVersion20 Version = "2.0"

var csafVersionPattern = alternativesUnmarshal(string(CSAFVersion20))

// TLP provides details about the TLP classification of the document.
type TLP struct {
	DocumentTLPLabel *TLPLabel `json:"label"` // required
	URL              *string   `json:"url,omitempty"`
}

// DocumentDistribution describes rules for sharing a document.
type DocumentDistribution struct {
	Text *string `json:"text,omitempty"`
	TLP  *TLP    `json:"tlp,omitempty"`
}

// DocumentPublisher provides information about the publishing entity.
type DocumentPublisher struct {
	Category         *Category `json:"category"` // required
	ContactDetails   *string   `json:"contact_details,omitempty"`
	IssuingAuthority *string   `json:"issuing_authority,omitempty"`
	Name             *string   `json:"name"`      // required
	Namespace        *string   `json:"namespace"` // required
}

// RevisionNumber specifies a version string to denote clearly the evolution of the content of the document.
type RevisionNumber string

var versionPattern = patternUnmarshal("^(0|[1-9][0-9]*)$|^((0|[1-9]\\d*)\\.(0|[1-9]\\d*)\\.(0|[1-9]\\d*)(?:-((?:0|[1-9]\\d*|\\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\\.(?:0|[1-9]\\d*|\\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\\+([0-9a-zA-Z-]+(?:\\.[0-9a-zA-Z-]+)*))?)$")

// Engine contains information about the engine that generated the CSAF document.
type Engine struct {
	Name    *string `json:"name"` // required
	Version *string `json:"version,omitempty"`
}

// Generator holds elements related to the generation of the document.
// These items will reference when the document was actually created,
// including the date it was generated and the entity that generated it.
type Generator struct {
	Date   *string `json:"date,omitempty"`
	Engine *Engine `json:"engine"` // required
}

// TrackingID is a unique identifier for the document.
type TrackingID string

var trackingIDPattern = patternUnmarshal("^[\\S](.*[\\S])?$")

// Revision contains information about one revision of the document.
type Revision struct {
	Date          *string         `json:"date"` // required
	LegacyVersion *string         `json:"legacy_version,omitempty"`
	Number        *RevisionNumber `json:"number"`  // required
	Summary       *string         `json:"summary"` // required
}

// TrackingStatus is the category of a publisher.
type TrackingStatus string

const (
	// CSAFTrackingStatusDraft is the "draft" category.
	CSAFTrackingStatusDraft TrackingStatus = "draft"
	// CSAFTrackingStatusFinal is the "final" category.
	CSAFTrackingStatusFinal TrackingStatus = "final"
	// CSAFTrackingStatusInterim is the "interim" category.
	CSAFTrackingStatusInterim TrackingStatus = "interim"
)

var csafTrackingStatusPattern = alternativesUnmarshal(
	string(CSAFTrackingStatusDraft),
	string(CSAFTrackingStatusFinal),
	string(CSAFTrackingStatusInterim))

// Revisions is a list of Revision.
type Revisions []*Revision

// Tracking holds information that is necessary to track a CSAF document.
type Tracking struct {
	Aliases            []*string       `json:"aliases,omitempty"`    // unique elements
	CurrentReleaseDate *string         `json:"current_release_date"` // required
	Generator          *Generator      `json:"generator"`
	ID                 *TrackingID     `json:"id"`                   // required
	InitialReleaseDate *string         `json:"initial_release_date"` // required
	RevisionHistory    Revisions       `json:"revision_history"`     // required
	Status             *TrackingStatus `json:"status"`               // required
	Version            *RevisionNumber `json:"version"`              // required
}

// Lang is a language identifier, corresponding to IETF BCP 47 / RFC 5646.
type Lang string

var langPattern = patternUnmarshal("^(([A-Za-z]{2,3}(-[A-Za-z]{3}(-[A-Za-z]{3}){0,2})?|[A-Za-z]{4,8})(-[A-Za-z]{4})?(-([A-Za-z]{2}|[0-9]{3}))?(-([A-Za-z0-9]{5,8}|[0-9][A-Za-z0-9]{3}))*(-[A-WY-Za-wy-z0-9](-[A-Za-z0-9]{2,8})+)*(-[Xx](-[A-Za-z0-9]{1,8})+)?|[Xx](-[A-Za-z0-9]{1,8})+|[Ii]-[Dd][Ee][Ff][Aa][Uu][Ll][Tt]|[Ii]-[Mm][Ii][Nn][Gg][Oo])$")

// Document contains meta-data about an advisory.
type Document struct {
	Acknowledgements  *Acknowledgements     `json:"acknowledgements,omitempty"`
	AggregateSeverity *AggregateSeverity    `json:"aggregate_severity,omitempty"`
	Category          *DocumentCategory     `json:"category"`     // required
	CSAFVersion       *Version              `json:"csaf_version"` // required
	Distribution      *DocumentDistribution `json:"distribution,omitempty"`
	Lang              *Lang                 `json:"lang,omitempty"`
	Notes             Notes                 `json:"notes,omitempty"`
	Publisher         *DocumentPublisher    `json:"publisher"` // required
	References        References            `json:"references,omitempty"`
	SourceLang        *Lang                 `json:"source_lang,omitempty"`
	Title             *string               `json:"title"`    // required
	Tracking          *Tracking             `json:"tracking"` // required
}

// ProductGroupID is a reference token for product group instances.
type ProductGroupID string

// ProductGroup is a group of products in the document that belong to one group.
type ProductGroup struct {
	GroupID    *string   `json:"group_id"`    // required
	ProductIDs *Products `json:"product_ids"` // required, two or more unique elements
	Summary    *string   `json:"summary,omitempty"`
}

// ProductGroups is a list of ProductGroupIDs
type ProductGroups struct {
	ProductGroupIDs []*ProductGroupID `json:"product_group_ids"` // unique elements
}

// RelationshipCategory is the category of a relationship.
type RelationshipCategory string

const (
	// CSAFRelationshipCategoryDefaultComponentOf is the "default_component_of" category.
	CSAFRelationshipCategoryDefaultComponentOf RelationshipCategory = "default_component_of"
	// CSAFRelationshipCategoryExternalComponentOf is the "external_component_of" category.
	CSAFRelationshipCategoryExternalComponentOf RelationshipCategory = "external_component_of"
	// CSAFRelationshipCategoryInstalledOn is the "installed_on" category.
	CSAFRelationshipCategoryInstalledOn RelationshipCategory = "installed_on"
	// CSAFRelationshipCategoryInstalledWith is the "installed_with" category.
	CSAFRelationshipCategoryInstalledWith RelationshipCategory = "installed_with"
	// CSAFRelationshipCategoryOptionalComponentOf is the "optional_component_of" category.
	CSAFRelationshipCategoryOptionalComponentOf RelationshipCategory = "optional_component_of"
)

var csafRelationshipCategoryPattern = alternativesUnmarshal(
	string(CSAFRelationshipCategoryDefaultComponentOf),
	string(CSAFRelationshipCategoryExternalComponentOf),
	string(CSAFRelationshipCategoryInstalledOn),
	string(CSAFRelationshipCategoryInstalledWith),
	string(CSAFRelationshipCategoryOptionalComponentOf))

// Relationship establishes a link between two existing FullProductName elements.
type Relationship struct {
	Category                  *RelationshipCategory `json:"category"`                     // required
	FullProductName           *FullProductName      `json:"full_product_name"`            // required
	ProductReference          *ProductID            `json:"product_reference"`            // required
	RelatesToProductReference *ProductID            `json:"relates_to_product_reference"` // required
}

// Relationships is a list of Relationship.
type Relationships []*Relationship

// Branches is a list of Branch.
type Branches []*Branch

// ProductTree contains product names that can be referenced elsewhere in the document.
type ProductTree struct {
	Branches         Branches          `json:"branches,omitempty"`
	FullProductNames *FullProductNames `json:"full_product_names,omitempty"`
	ProductGroups    *ProductGroups    `json:"product_groups,omitempty"`
	RelationShips    *Relationships    `json:"relationships,omitempty"`
}

// CVE holds the MITRE standard Common Vulnerabilities and Exposures (CVE) tracking number for a vulnerability.
type CVE string

var cvePattern = patternUnmarshal("^CVE-[0-9]{4}-[0-9]{4,}$")

// WeaknessID is the identifier of a weakness.
type WeaknessID string

var weaknessIDPattern = patternUnmarshal("^CWE-[1-9]\\d{0,5}$")

// CWE holds the MITRE standard Common Weakness Enumeration (CWE) for the weakness associated.
type CWE struct {
	ID   *WeaknessID `json:"id"`   // required
	Name *string     `json:"name"` // required
}

// FlagLabel is the label of a flag for a vulnerability.
type FlagLabel string

const (
	// CSAFFlagLabelComponentNotPresent is the "component_not_present" label.
	CSAFFlagLabelComponentNotPresent FlagLabel = "component_not_present"
	// CSAFFlagLabelInlineMitigationsAlreadyExist is the "inline_mitigations_already_exist" label.
	CSAFFlagLabelInlineMitigationsAlreadyExist FlagLabel = "inline_mitigations_already_exist"
	// CSAFFlagLabelVulnerableCodeCannotBeControlledByAdversary is the "vulnerable_code_cannot_be_controlled_by_adversary" label.
	CSAFFlagLabelVulnerableCodeCannotBeControlledByAdversary FlagLabel = "vulnerable_code_cannot_be_controlled_by_adversary"
	// CSAFFlagLabelVulnerableCodeNotInExecutePath is the "vulnerable_code_not_in_execute_path" label.
	CSAFFlagLabelVulnerableCodeNotInExecutePath FlagLabel = "vulnerable_code_not_in_execute_path"
	// CSAFFlagLabelVulnerableCodeNotPresent is the "vulnerable_code_not_present" label.
	CSAFFlagLabelVulnerableCodeNotPresent FlagLabel = "vulnerable_code_not_present"
)

var csafFlagLabelPattern = alternativesUnmarshal(
	string(CSAFFlagLabelComponentNotPresent),
	string(CSAFFlagLabelInlineMitigationsAlreadyExist),
	string(CSAFFlagLabelVulnerableCodeCannotBeControlledByAdversary),
	string(CSAFFlagLabelVulnerableCodeNotInExecutePath),
	string(CSAFFlagLabelVulnerableCodeNotPresent))

// Flag contains product specific information in regard to this vulnerability as a single
// machine readable flag. For example, this could be a machine readable justification
// code why a product is not affected.
type Flag struct {
	Date     *string        `json:"date,omitempty"`
	GroupIDs *ProductGroups `json:"group_ids,omitempty"`
	Label    *FlagLabel     `json:"label"` // required
	//revive:disable-next-line:var-naming  until new major version w fix
	ProductIds *Products `json:"product_ids,omitempty"`
}

// Flags is a list if Flag elements.
type Flags []*Flag

// VulnerabilityID is the identifier of a vulnerability.
type VulnerabilityID struct {
	SystemName *string `json:"system_name"` // required
	Text       *string `json:"text"`        // required
}

// VulnerabilityIDs is a list of VulnerabilityID elements.
type VulnerabilityIDs []*VulnerabilityID

// InvolvementParty is the party of an involvement.
type InvolvementParty string

const (
	// CSAFInvolvementPartyCoordinator is the "coordinator" party.
	CSAFInvolvementPartyCoordinator InvolvementParty = "coordinator"
	// CSAFInvolvementPartyDiscoverer is the "discoverer" party.
	CSAFInvolvementPartyDiscoverer InvolvementParty = "discoverer"
	// CSAFInvolvementPartyOther is the "other" party.
	CSAFInvolvementPartyOther InvolvementParty = "other"
	// CSAFInvolvementPartyUser is the "user" party.
	CSAFInvolvementPartyUser InvolvementParty = "user"
	// CSAFInvolvementPartyVendor is the "vendor" party.
	CSAFInvolvementPartyVendor InvolvementParty = "vendor"
)

var csafInvolvementPartyPattern = alternativesUnmarshal(
	string(CSAFInvolvementPartyCoordinator),
	string(CSAFInvolvementPartyDiscoverer),
	string(CSAFInvolvementPartyOther),
	string(CSAFInvolvementPartyUser),
	string(CSAFInvolvementPartyVendor))

// InvolvementStatus is the status of an involvement.
type InvolvementStatus string

const (
	// CSAFInvolvementStatusCompleted is the "completed" status.
	CSAFInvolvementStatusCompleted InvolvementStatus = "completed"
	// CSAFInvolvementStatusContactAttempted is the "contact_attempted" status.
	CSAFInvolvementStatusContactAttempted InvolvementStatus = "contact_attempted"
	// CSAFInvolvementStatusDisputed is the "disputed" status.
	CSAFInvolvementStatusDisputed InvolvementStatus = "disputed"
	// CSAFInvolvementStatusInProgress is the "in_progress" status.
	CSAFInvolvementStatusInProgress InvolvementStatus = "in_progress"
	// CSAFInvolvementStatusNotContacted is the "not_contacted" status.
	CSAFInvolvementStatusNotContacted InvolvementStatus = "not_contacted"
	// CSAFInvolvementStatusOpen is the "open" status.
	CSAFInvolvementStatusOpen InvolvementStatus = "open"
)

var csafInvolvementStatusPattern = alternativesUnmarshal(
	string(CSAFInvolvementStatusCompleted),
	string(CSAFInvolvementStatusContactAttempted),
	string(CSAFInvolvementStatusDisputed),
	string(CSAFInvolvementStatusInProgress),
	string(CSAFInvolvementStatusNotContacted),
	string(CSAFInvolvementStatusOpen))

// Involvement is a container that allows the document producers to comment on the level of involvement
// (or engagement) of themselves (or third parties) in the vulnerability identification, scoping, and
// remediation process. It can also be used to convey the disclosure timeline.
// The ordered tuple of the values of party and date (if present) SHALL be unique within the involvements
// of a vulnerability.
type Involvement struct {
	Date    *string            `json:"date,omitempty"`
	Party   *InvolvementParty  `json:"party"`  // required
	Status  *InvolvementStatus `json:"status"` // required
	Summary *string            `json:"summary,omitempty"`
}

// Involvements is a list of Involvement elements.
type Involvements []*Involvement

// ProductStatus contains different lists of ProductIDs which provide details on
// the status of the referenced product related to the current vulnerability.
type ProductStatus struct {
	FirstAffected      *Products `json:"first_affected,omitempty"`
	FirstFixed         *Products `json:"first_fixed,omitempty"`
	Fixed              *Products `json:"fixed,omitempty"`
	KnownAffected      *Products `json:"known_affected,omitempty"`
	KnownNotAffected   *Products `json:"known_not_affected,omitempty"`
	LastAffected       *Products `json:"last_affected,omitempty"`
	Recommended        *Products `json:"recommended,omitempty"`
	UnderInvestigation *Products `json:"under_investigation,omitempty"`
}

// RemediationCategory is the category of a remediation.
type RemediationCategory string

const (
	// CSAFRemediationCategoryMitigation is the "mitigation" category.
	CSAFRemediationCategoryMitigation RemediationCategory = "mitigation"
	// CSAFRemediationCategoryNoFixPlanned is the "no_fix_planned" category.
	CSAFRemediationCategoryNoFixPlanned RemediationCategory = "no_fix_planned"
	// CSAFRemediationCategoryNoneAvailable is the "none_available" category.
	CSAFRemediationCategoryNoneAvailable RemediationCategory = "none_available"
	// CSAFRemediationCategoryVendorFix is the "vendor_fix" category.
	CSAFRemediationCategoryVendorFix RemediationCategory = "vendor_fix"
	// CSAFRemediationCategoryWorkaround is the "workaround" category.
	CSAFRemediationCategoryWorkaround RemediationCategory = "workaround"
)

var csafRemediationCategoryPattern = alternativesUnmarshal(
	string(CSAFRemediationCategoryMitigation),
	string(CSAFRemediationCategoryNoFixPlanned),
	string(CSAFRemediationCategoryNoneAvailable),
	string(CSAFRemediationCategoryVendorFix),
	string(CSAFRemediationCategoryWorkaround))

// RestartRequiredCategory is the category of RestartRequired.
type RestartRequiredCategory string

const (
	// CSAFRestartRequiredCategoryConnected is the "connected" category.
	CSAFRestartRequiredCategoryConnected RestartRequiredCategory = "connected"
	// CSAFRestartRequiredCategoryDependencies is the "dependencies" category.
	CSAFRestartRequiredCategoryDependencies RestartRequiredCategory = "dependencies"
	// CSAFRestartRequiredCategoryMachine is the "machine" category.
	CSAFRestartRequiredCategoryMachine RestartRequiredCategory = "machine"
	// CSAFRestartRequiredCategoryNone is the "none" category.
	CSAFRestartRequiredCategoryNone RestartRequiredCategory = "none"
	// CSAFRestartRequiredCategoryParent is the "parent" category.
	CSAFRestartRequiredCategoryParent RestartRequiredCategory = "parent"
	// CSAFRestartRequiredCategoryService is the "service" category.
	CSAFRestartRequiredCategoryService RestartRequiredCategory = "service"
	// CSAFRestartRequiredCategorySystem is the "system" category.
	CSAFRestartRequiredCategorySystem RestartRequiredCategory = "system"
	// CSAFRestartRequiredCategoryVulnerableComponent is the "vulnerable_component" category.
	CSAFRestartRequiredCategoryVulnerableComponent RestartRequiredCategory = "vulnerable_component"
	// CSAFRestartRequiredCategoryZone is the "zone" category.
	CSAFRestartRequiredCategoryZone RestartRequiredCategory = "zone"
)

var csafRestartRequiredCategoryPattern = alternativesUnmarshal(
	string(CSAFRestartRequiredCategoryConnected),
	string(CSAFRestartRequiredCategoryDependencies),
	string(CSAFRestartRequiredCategoryMachine),
	string(CSAFRestartRequiredCategoryNone),
	string(CSAFRestartRequiredCategoryParent),
	string(CSAFRestartRequiredCategoryService),
	string(CSAFRestartRequiredCategorySystem),
	string(CSAFRestartRequiredCategoryVulnerableComponent),
	string(CSAFRestartRequiredCategoryZone))

// RestartRequired provides information on category of restart is required by this remediation to become
// effective.
type RestartRequired struct {
	Category *RestartRequiredCategory `json:"category"` // required
	Details  *string                  `json:"details,omitempty"`
}

// Remediation specifies details on how to handle (and presumably, fix) a vulnerability.
type Remediation struct {
	Category     *RemediationCategory `json:"category"` // required
	Date         *string              `json:"date,omitempty"`
	Details      *string              `json:"details"` // required
	Entitlements []*string            `json:"entitlements,omitempty"`
	//revive:disable:var-naming until new major version w fix
	GroupIds   *ProductGroups `json:"group_ids,omitempty"`
	ProductIds *Products      `json:"product_ids,omitempty"`
	//revive:enable
	RestartRequired *RestartRequired `json:"restart_required,omitempty"`
	URL             *string          `json:"url,omitempty"`
}

// Remediations is a list of Remediation elements.
type Remediations []*Remediation

// CVSSVersion2 is the version of a CVSS2 item.
type CVSSVersion2 string

// CVSSVersion20 is the current version of the schema.
const CVSSVersion20 CVSSVersion2 = "2.0"

var cvssVersion2Pattern = alternativesUnmarshal(string(CVSSVersion20))

// CVSS2VectorString is the VectorString of a CVSS2 item with version 3.x.
type CVSS2VectorString string

var cvss2VectorStringPattern = patternUnmarshal(`^((AV:[NAL]|AC:[LMH]|Au:[MSN]|[CIA]:[NPC]|E:(U|POC|F|H|ND)|RL:(OF|TF|W|U|ND)|RC:(UC|UR|C|ND)|CDP:(N|L|LM|MH|H|ND)|TD:(N|L|M|H|ND)|[CIA]R:(L|M|H|ND))/)*(AV:[NAL]|AC:[LMH]|Au:[MSN]|[CIA]:[NPC]|E:(U|POC|F|H|ND)|RL:(OF|TF|W|U|ND)|RC:(UC|UR|C|ND)|CDP:(N|L|LM|MH|H|ND)|TD:(N|L|M|H|ND)|[CIA]R:(L|M|H|ND))$`)

// CVSSVersion3 is the version of a CVSS3 item.
type CVSSVersion3 string

// CVSSVersion30 is version 3.0 of a CVSS3 item.
const CVSSVersion30 CVSSVersion3 = "3.0"

// CVSSVersion31 is version 3.1 of a CVSS3 item.
const CVSSVersion31 CVSSVersion3 = "3.1"

var cvss3VersionPattern = alternativesUnmarshal(
	string(CVSSVersion30),
	string(CVSSVersion31))

// CVSS3VectorString is the VectorString of a CVSS3 item with version 3.x.
type CVSS3VectorString string

// cvss3VectorStringPattern is a combination of the vectorString patterns of CVSS 3.0
// and CVSS 3.1 since the only difference is the number directly after the first dot.
var cvss3VectorStringPattern = patternUnmarshal(`^CVSS:3[.][01]/((AV:[NALP]|AC:[LH]|PR:[NLH]|UI:[NR]|S:[UC]|[CIA]:[NLH]|E:[XUPFH]|RL:[XOTWU]|RC:[XURC]|[CIA]R:[XLMH]|MAV:[XNALP]|MAC:[XLH]|MPR:[XNLH]|MUI:[XNR]|MS:[XUC]|M[CIA]:[XNLH])/)*(AV:[NALP]|AC:[LH]|PR:[NLH]|UI:[NR]|S:[UC]|[CIA]:[NLH]|E:[XUPFH]|RL:[XOTWU]|RC:[XURC]|[CIA]R:[XLMH]|MAV:[XNALP]|MAC:[XLH]|MPR:[XNLH]|MUI:[XNR]|MS:[XUC]|M[CIA]:[XNLH])$`)

// CVSS2 holding a CVSS v2.0 value
type CVSS2 struct {
	Version                    *CVSSVersion2                    `json:"version"`      // required
	VectorString               *CVSS2VectorString               `json:"vectorString"` // required
	AccessVector               *CVSS20AccessVector              `json:"accessVector,omitempty"`
	AccessComplexity           *CVSS20AccessComplexity          `json:"accessComplexity,omitempty"`
	Authentication             *CVSS20Authentication            `json:"authentication,omitempty"`
	ConfidentialityImpact      *CVSS20Cia                       `json:"confidentialityImpact,omitempty"`
	IntegrityImpact            *CVSS20Cia                       `json:"integrityImpact,omitempty"`
	AvailabilityImpact         *CVSS20Cia                       `json:"availabilityImpact,omitempty"`
	BaseScore                  *float64                         `json:"baseScore"` // required
	Exploitability             *CVSS20Exploitability            `json:"exploitability,omitempty"`
	RemediationLevel           *CVSS20RemediationLevel          `json:"remediationLevel,omitempty"`
	ReportConfidence           *CVSS20ReportConfidence          `json:"reportConfidence,omitempty"`
	TemporalScore              *float64                         `json:"temporalScore,omitempty"`
	CollateralDamagePotential  *CVSS20CollateralDamagePotential `json:"collateralDamagePotential,omitempty"`
	TargetDistribution         *CVSS20TargetDistribution        `json:"targetDistribution,omitempty"`
	ConfidentialityRequirement *CVSS20CiaRequirement            `json:"confidentialityRequirement,omitempty"`
	IntegrityRequirement       *CVSS20CiaRequirement            `json:"integrityRequirement,omitempty"`
	AvailabilityRequirement    *CVSS20CiaRequirement            `json:"availabilityRequirement,omitempty"`
	EnvironmentalScore         *float64                         `json:"environmentalScore,omitempty"`
}

// CVSS3 holding a CVSS v3.x value
type CVSS3 struct {
	Version                       *CVSSVersion3                    `json:"version"`      // required
	VectorString                  *CVSS3VectorString               `json:"vectorString"` // required
	AttackVector                  *CVSS3AttackVector               `json:"attackVector,omitempty"`
	AttackComplexity              *CVSS3AttackComplexity           `json:"attackComplexity,omitempty"`
	PrivilegesRequired            *CVSS3PrivilegesRequired         `json:"privilegesRequired,omitempty"`
	UserInteraction               *CVSS3UserInteraction            `json:"userInteraction,omitempty"`
	Scope                         *CVSS3Scope                      `json:"scope,omitempty"`
	ConfidentialityImpact         *CVSS3Cia                        `json:"confidentialityImpact,omitempty"`
	IntegrityImpact               CVSS3Cia                         `json:"integrityImpact,omitempty"`
	AvailabilityImpact            *CVSS3Cia                        `json:"availabilityImpact,omitempty"`
	BaseScore                     *float64                         `json:"baseScore"`    // required
	BaseSeverity                  *CVSS3Severity                   `json:"baseSeverity"` // required
	ExploitCodeMaturity           *CVSS3ExploitCodeMaturity        `json:"exploitCodeMaturity,omitempty"`
	RemediationLevel              *CVSS3RemediationLevel           `json:"remediationLevel,omitempty"`
	ReportConfidence              *CVSS3Confidence                 `json:"reportConfidence,omitempty"`
	TemporalScore                 *float64                         `json:"temporalScore,omitempty"`
	TemporalSeverity              *CVSS3Severity                   `json:"temporalSeverity,omitempty"`
	ConfidentialityRequirement    *CVSS3CiaRequirement             `json:"confidentialityRequirement,omitempty"`
	IntegrityRequirement          *CVSS3CiaRequirement             `json:"integrityRequirement,omitempty"`
	AvailabilityRequirement       *CVSS3CiaRequirement             `json:"availabilityRequirement,omitempty"`
	ModifiedAttackVector          *CVSS3ModifiedAttackVector       `json:"modifiedAttackVector,omitempty"`
	ModifiedAttackComplexity      *CVSS3ModifiedAttackComplexity   `json:"modifiedAttackComplexity,omitempty"`
	ModifiedPrivilegesRequired    *CVSS3ModifiedPrivilegesRequired `json:"modifiedPrivilegesRequired,omitempty"`
	ModifiedUserInteraction       *CVSS3ModifiedUserInteraction    `json:"modifiedUserInteraction,omitempty"`
	ModifiedScope                 *CVSS3ModifiedScope              `json:"modifiedScope,omitempty"`
	ModifiedConfidentialityImpact *CVSS3ModifiedCia                `json:"modifiedConfidentialityImpact,omitempty"`
	ModifiedIntegrityImpact       *CVSS3ModifiedCia                `json:"modifiedIntegrityImpact,omitempty"`
	ModifiedAvailabilityImpact    *CVSS3ModifiedCia                `json:"modifiedAvailabilityImpact,omitempty"`
	EenvironmentalScore           *float64                         `json:"environmentalScore,omitempty"`
	EnvironmentalSeverity         *CVSS3Severity                   `json:"environmentalSeverity,omitempty"`
}

// Score specifies information about (at least one) score of the vulnerability and for which
// products the given value applies. A Score item has at least 2 properties.
type Score struct {
	CVSS2    *CVSS2    `json:"cvss_v2,omitempty"`
	CVSS3    *CVSS3    `json:"cvss_v3,omitempty"`
	Products *Products `json:"products"` // required
}

// Scores is a list of Score elements.
type Scores []*Score

// ThreatCategory is the category of a threat.
type ThreatCategory string

const (
	// CSAFThreatCategoryExploitStatus is the "exploit_status" category.
	CSAFThreatCategoryExploitStatus ThreatCategory = "exploit_status"
	// CSAFThreatCategoryImpact is the "impact" category.
	CSAFThreatCategoryImpact ThreatCategory = "impact"
	// CSAFThreatCategoryTargetSet is the "target_set" category.
	CSAFThreatCategoryTargetSet ThreatCategory = "target_set"
)

var csafThreatCategoryPattern = alternativesUnmarshal(
	string(CSAFThreatCategoryExploitStatus),
	string(CSAFThreatCategoryImpact),
	string(CSAFThreatCategoryTargetSet))

// Threat contains information about a vulnerability that can change with time.
type Threat struct {
	Category *ThreatCategory `json:"category"` // required
	Date     *string         `json:"date,omitempty"`
	Details  *string         `json:"details"` // required
	//revive:disable:var-naming until new major version w fix
	GroupIds   *ProductGroups `json:"group_ids,omitempty"`
	ProductIds *Products      `json:"product_ids,omitempty"`
	//revive:enable
}

// Threats is a list of Threat elements.
type Threats []*Threat

// Notes is a list of Note.
type Notes []*Note

// References is a list of Reference.
type References []*Reference

// Vulnerability contains all fields that are related to a single vulnerability in the document.
type Vulnerability struct {
	Acknowledgements Acknowledgements `json:"acknowledgements,omitempty"`
	CVE              *CVE             `json:"cve,omitempty"`
	CWE              *CWE             `json:"cwe,omitempty"`
	DiscoveryDate    *string          `json:"discovery_date,omitempty"`
	Flags            Flags            `json:"flags,omitempty"`
	IDs              VulnerabilityIDs `json:"ids,omitempty"` // unique ID elements
	Involvements     Involvements     `json:"involvements,omitempty"`
	Notes            Notes            `json:"notes,omitempty"`
	ProductStatus    *ProductStatus   `json:"product_status,omitempty"`
	References       References       `json:"references,omitempty"`
	ReleaseDate      *string          `json:"release_date,omitempty"`
	Remediations     Remediations     `json:"remediations,omitempty"`
	Scores           Scores           `json:"scores,omitempty"`
	Threats          Threats          `json:"threats,omitempty"`
	Title            *string          `json:"title,omitempty"`
}

// Vulnerabilities is a list of Vulnerability
type Vulnerabilities []*Vulnerability

// Advisory represents a CSAF advisory.
type Advisory struct {
	Document        *Document       `json:"document"` // required
	ProductTree     *ProductTree    `json:"product_tree,omitempty"`
	Vulnerabilities Vulnerabilities `json:"vulnerabilities,omitempty"`
}

// Validate validates a AggregateSeverity.
func (as *AggregateSeverity) Validate() error {
	if as.Text == nil {
		return errors.New("'text' is missing")
	}
	return nil
}

// Validate validates a DocumentDistribution.
func (dd *DocumentDistribution) Validate() error {
	if dd.Text == nil && dd.TLP == nil {
		return errors.New("needs at least properties 'text' or 'tlp'")
	}
	return nil
}

// Validate validates a list of notes.
func (ns Notes) Validate() error {
	for i, n := range ns {
		if err := n.Validate(); err != nil {
			return fmt.Errorf("%d. note is invalid: %w", i+1, err)
		}
	}
	return nil
}

// Validate validates a single note.
func (n *Note) Validate() error {
	switch {
	case n == nil:
		return errors.New("is nil")
	case n.NoteCategory == nil:
		return errors.New("'note_category' is missing")
	case n.Text == nil:
		return errors.New("'text' is missing")
	default:
		return nil
	}
}

// Validate validates a DocumentPublisher.
func (p *DocumentPublisher) Validate() error {
	switch {
	case p.Category == nil:
		return errors.New("'document' is missing")
	case p.Name == nil:
		return errors.New("'name' is missing")
	case p.Namespace == nil:
		return errors.New("'namespace' is missing")
	default:
		return nil
	}
}

// Validate validates a single reference.
func (r *Reference) Validate() error {
	switch {
	case r.Summary == nil:
		return errors.New("summary' is missing")
	case r.URL == nil:
		return errors.New("'url' is missing")
	default:
		return nil
	}
}

// Validate validates a list of references.
func (rs References) Validate() error {
	for i, r := range rs {
		if err := r.Validate(); err != nil {
			return fmt.Errorf("%d. reference is invalid: %w", i+1, err)
		}
	}
	return nil
}

// Validate validates a single revision.
func (r *Revision) Validate() error {
	switch {
	case r.Date == nil:
		return errors.New("'date' is missing")
	case r.Number == nil:
		return errors.New("'number' is missing")
	case r.Summary == nil:
		return errors.New("'summary' is missing")
	default:
		return nil
	}
}

// Validate validates a list of revisions.
func (rs Revisions) Validate() error {
	for i, r := range rs {
		if err := r.Validate(); err != nil {
			return fmt.Errorf("%d. revision is invalid: %w", i+1, err)
		}
	}
	return nil
}

// Validate validates an Engine.
func (e *Engine) Validate() error {
	if e.Version == nil {
		return errors.New("'version' is missing")
	}
	return nil
}

// Validate validates a Generator.
func (g *Generator) Validate() error {
	if g.Engine == nil {
		return errors.New("'engine' is missing")
	}
	if err := g.Engine.Validate(); err != nil {
		return fmt.Errorf("'engine' is invalid: %w", err)
	}
	return nil
}

// Validate validates a single Tracking.
func (t *Tracking) Validate() error {
	switch {
	case t.CurrentReleaseDate == nil:
		return errors.New("'current_release_date' is missing")
	case t.ID == nil:
		return errors.New("'id' is missing")
	case t.InitialReleaseDate == nil:
		return errors.New("'initial_release_date' is missing")
	case t.RevisionHistory == nil:
		return errors.New("'revision_history' is missing")
	case t.Status == nil:
		return errors.New("'status' is missing")
	case t.Version == nil:
		return errors.New("'version' is missing")
	}
	if err := t.RevisionHistory.Validate(); err != nil {
		return fmt.Errorf("'revision_history' is invalid: %w", err)
	}
	if t.Generator != nil {
		if err := t.Generator.Validate(); err != nil {
			return fmt.Errorf("'generator' is invalid: %w", err)
		}
	}
	return nil
}

// Validate validates a Document.
func (doc *Document) Validate() error {
	switch {
	case doc.Category == nil:
		return errors.New("'category' is missing")
	case doc.CSAFVersion == nil:
		return errors.New("'csaf_version' is missing")
	case doc.Publisher == nil:
		return errors.New("'publisher' is missing")
	case doc.Title == nil:
		return errors.New("'title' is missing")
	case doc.Tracking == nil:
		return errors.New("'tracking' is missing")
	}
	if err := doc.Tracking.Validate(); err != nil {
		return fmt.Errorf("'tracking' is invalid: %w", err)
	}
	if doc.Distribution != nil {
		if err := doc.Distribution.Validate(); err != nil {
			return fmt.Errorf("'distribution' is invalid: %w", err)
		}
	}
	if doc.AggregateSeverity != nil {
		if err := doc.AggregateSeverity.Validate(); err != nil {
			return fmt.Errorf("'aggregate_severity' is invalid: %w", err)
		}
	}
	if err := doc.Publisher.Validate(); err != nil {
		return fmt.Errorf("'publisher' is invalid: %w", err)
	}
	if err := doc.References.Validate(); err != nil {
		return fmt.Errorf("'references' is invalid: %w", err)
	}
	if err := doc.Notes.Validate(); err != nil {
		return fmt.Errorf("'notes' is invalid: %w", err)
	}
	return nil
}

// Validate validates a single FileHash.
func (fh *FileHash) Validate() error {
	switch {
	case fh == nil:
		return errors.New("is nil")
	case fh.Algorithm == nil:
		return errors.New("'algorithm' is missing")
	case fh.Value == nil:
		return errors.New("'value' is missing")
	default:
		return nil
	}
}

// Validate validates a list of file hashes.
func (hs *Hashes) Validate() error {
	switch {
	case hs.FileHashes == nil:
		return errors.New("'hashes' is missing")
	case hs.FileName == nil:
		return errors.New("'filename' is missing")
	}
	for i, fh := range hs.FileHashes {
		if err := fh.Validate(); err != nil {
			return fmt.Errorf("%d. file hash is invalid: %w", i+1, err)
		}
	}
	return nil
}

// Validate validates a single XGenericURI.
func (xgu *XGenericURI) Validate() error {
	switch {
	case xgu == nil:
		return errors.New("is nil")
	case xgu.Namespace == nil:
		return errors.New("'namespace' is missing")
	case xgu.URI == nil:
		return errors.New("'uri' is missing")
	default:
		return nil
	}
}

// Validate validates a list of XGenericURIs.
func (xgus XGenericURIs) Validate() error {
	for i, xgu := range xgus {
		if err := xgu.Validate(); err != nil {
			return fmt.Errorf("%d. generic uri is invalid: %w", i+1, err)
		}
	}
	return nil
}

// Validate validates a ProductIdentificationHelper.
func (pih *ProductIdentificationHelper) Validate() error {
	if pih.Hashes != nil {
		if err := pih.Hashes.Validate(); err != nil {
			return fmt.Errorf("'hashes' is invalid: %w", err)
		}
	}
	if pih.XGenericURIs != nil {
		if err := pih.XGenericURIs.Validate(); err != nil {
			return fmt.Errorf("'x_generic_uris' is invalid: %w", err)
		}
	}
	return nil
}

// Validate validates a FullProductName.
func (fpn *FullProductName) Validate() error {
	switch {
	case fpn.Name == nil:
		return errors.New("'name' is missing")
	case fpn.ProductID == nil:
		return errors.New("'product_id' is missing")
	}
	if fpn.ProductIdentificationHelper != nil {
		if err := fpn.ProductIdentificationHelper.Validate(); err != nil {
			return fmt.Errorf("'product_identification_helper' is invalid: %w", err)
		}
	}
	return nil
}

// Validate validates a list of Relationship elements.
func (fpns FullProductNames) Validate() error {
	for i, f := range fpns {
		if err := f.Validate(); err != nil {
			return fmt.Errorf("%d. full product name is invalid: %w", i+1, err)
		}
	}
	return nil
}

// Validate validates a single Branch.
func (b *Branch) Validate() error {
	switch {
	case b.Category == nil:
		return errors.New("'category' is missing")
	case b.Name == nil:
		return errors.New("'name' is missing")
	}
	if b.Product != nil {
		if err := b.Product.Validate(); err != nil {
			return fmt.Errorf("'product' is invalid: %w", err)
		}
	}
	return b.Branches.Validate()
}

// Validate validates a single Relationship.
func (r *Relationship) Validate() error {
	switch {
	case r.Category == nil:
		return errors.New("'category' is missing")
	case r.ProductReference == nil:
		return errors.New("'product_reference' is missing")
	case r.RelatesToProductReference == nil:
		return errors.New("'relates_to_product_reference' is missing")
	}
	if r.FullProductName != nil {
		if err := r.FullProductName.Validate(); err != nil {
			return fmt.Errorf("'product' is invalid: %w", err)
		}
	}
	return nil
}

// Validate validates a list of branches.
func (bs Branches) Validate() error {
	for i, b := range bs {
		if err := b.Validate(); err != nil {
			return fmt.Errorf("%d. branch is invalid: %w", i+1, err)
		}
	}
	return nil
}

// Validate validates a list of Relationship elements.
func (rs Relationships) Validate() error {
	for i, r := range rs {
		if err := r.Validate(); err != nil {
			return fmt.Errorf("%d. relationship is invalid: %w", i+1, err)
		}
	}
	return nil
}

// Validate validates a ProductTree.
func (pt *ProductTree) Validate() error {
	if err := pt.Branches.Validate(); err != nil {
		return fmt.Errorf("'branches' is invalid: %w", err)
	}
	if pt.FullProductNames != nil {
		if err := pt.FullProductNames.Validate(); err != nil {
			return fmt.Errorf("'full_product_names is invalid: %w", err)
		}
	}
	if pt.RelationShips != nil {
		if err := pt.RelationShips.Validate(); err != nil {
			return fmt.Errorf("'relationships' is invalid: %w", err)
		}
	}
	return nil
}

// Validate validates a single Flag.
func (f *Flag) Validate() error {
	if f.Label == nil {
		return errors.New("'label' is missing")
	}
	return nil
}

// Validate validates a list of Flag elements.
func (fs Flags) Validate() error {
	for i, f := range fs {
		if err := f.Validate(); err != nil {
			return fmt.Errorf("%d. flag is invalid: %w", i+1, err)
		}
	}
	return nil
}

// Validate validates a CWE.
func (cwe *CWE) Validate() error {
	switch {
	case cwe.ID == nil:
		return errors.New("'id' is missing")
	case cwe.Name == nil:
		return errors.New("'name' is missing")
	}
	return nil
}

// Validate validates a single VulnerabilityID.
func (id *VulnerabilityID) Validate() error {
	switch {
	case id.SystemName == nil:
		return errors.New("'system_name' is missing")
	case id.Text == nil:
		return errors.New("'text' is missing")
	}
	return nil
}

// Validate validates a list of VulnerabilityID elements.
func (ids VulnerabilityIDs) Validate() error {
	for i, id := range ids {
		if err := id.Validate(); err != nil {
			return fmt.Errorf("%d. vulnerability id is invalid: %w", i+1, err)
		}
	}
	return nil
}

// Validate validates a single Involvement.
func (iv *Involvement) Validate() error {
	switch {
	case iv.Party == nil:
		return errors.New("'party' is missing")
	case iv.Status == nil:
		return errors.New("'status' is missing")
	}
	return nil
}

// Validate validates a list of Involvement elements.
func (ivs Involvements) Validate() error {
	for i, iv := range ivs {
		if err := iv.Validate(); err != nil {
			return fmt.Errorf("%d. involvement is invalid: %w", i+1, err)
		}
	}
	return nil
}

// Validate validates a RestartRequired.
func (rr *RestartRequired) Validate() error {
	if rr.Category == nil {
		return errors.New("'category' is missing")
	}
	return nil
}

// Validate validates a CVSS2
func (c *CVSS2) Validate() error {
	switch {
	case c.Version == nil:
		return errors.New("'version' is missing")
	case c.VectorString == nil:
		return errors.New("'vectorString' is missing")
	case c.BaseScore == nil:
		return errors.New("'baseScore' is missing")
	}
	return nil
}

// Validate validates a CVSS3
func (c *CVSS3) Validate() error {
	switch {
	case c.Version == nil:
		return errors.New("'version' is missing")
	case c.VectorString == nil:
		return errors.New("'vectorString' is missing")
	case c.BaseScore == nil:
		return errors.New("'baseScore' is missing")
	case c.BaseSeverity == nil:
		return errors.New("'baseSeverity' is missing")
	}
	return nil
}

// Validate validates a single Score.
func (s *Score) Validate() error {
	if s.Products == nil {
		return errors.New("'products' is missing")
	}
	if s.CVSS2 != nil {
		if err := s.CVSS2.Validate(); err != nil {
			return fmt.Errorf("'cvss_v2' is invalid: %w", err)
		}
	}
	if s.CVSS3 != nil {
		if err := s.CVSS3.Validate(); err != nil {
			return fmt.Errorf("'cvss_v3' is invalid: %w", err)
		}
	}
	return nil
}

// Validate validates a list of Score elements.
func (ss Scores) Validate() error {
	for i, s := range ss {
		if err := s.Validate(); err != nil {
			return fmt.Errorf("%d. score is invalid: %w", i+1, err)
		}
	}
	return nil
}

// Validate validates a single Remediation.
func (r *Remediation) Validate() error {
	switch {
	case r.Category == nil:
		return errors.New("'category' is missing")
	case r.Details == nil:
		return errors.New("'details' is missing")
	}
	if r.RestartRequired != nil {
		if err := r.RestartRequired.Validate(); err != nil {
			return fmt.Errorf("'restart_required' is invalid: %w", err)
		}
	}
	return nil
}

// Validate validates a list of Remediation elements.
func (rms Remediations) Validate() error {
	for i, r := range rms {
		if err := r.Validate(); err != nil {
			return fmt.Errorf("%d. remediation is invalid: %w", i+1, err)
		}
	}
	return nil
}

// Validate validates a single Threat.
func (t *Threat) Validate() error {
	switch {
	case t.Category == nil:
		return errors.New("'category' is missing")
	case t.Details == nil:
		return errors.New("'details' is missing")
	}
	return nil
}

// Validate validates a list of Threat elements.
func (ts Threats) Validate() error {
	for i, t := range ts {
		if err := t.Validate(); err != nil {
			return fmt.Errorf("%d. threat is invalid: %w", i+1, err)
		}
	}
	return nil
}

// Validate validates a single Vulnerability.
func (v *Vulnerability) Validate() error {
	if v.CWE != nil {
		if err := v.CWE.Validate(); err != nil {
			return fmt.Errorf("'cwe' is invalid: %w", err)
		}
	}
	if err := v.Flags.Validate(); err != nil {
		return fmt.Errorf("'flags' is invalid: %w", err)
	}
	if err := v.IDs.Validate(); err != nil {
		return fmt.Errorf("'ids' is invalid: %w", err)
	}
	if err := v.Involvements.Validate(); err != nil {
		return fmt.Errorf("'involvements' is invalid: %w", err)
	}
	if err := v.Notes.Validate(); err != nil {
		return fmt.Errorf("'notes' is invalid: %w", err)
	}
	if err := v.References.Validate(); err != nil {
		return fmt.Errorf("'references' is invalid: %w", err)
	}
	if err := v.Remediations.Validate(); err != nil {
		return fmt.Errorf("'remediations' is invalid: %w", err)
	}
	if err := v.Scores.Validate(); err != nil {
		return fmt.Errorf("'scores' is invalid: %w", err)
	}
	if err := v.Threats.Validate(); err != nil {
		return fmt.Errorf("'threats' is invalid: %w", err)
	}
	return nil
}

// Validate validates a list of Vulnerability elements.
func (vs Vulnerabilities) Validate() error {
	for i, v := range vs {
		if err := v.Validate(); err != nil {
			return fmt.Errorf("%d. vulnerability is invalid: %w", i+1, err)
		}
	}
	return nil
}

// Validate checks if the advisory is valid.
// Returns an error if the validation fails otherwise nil.
func (adv *Advisory) Validate() error {
	if adv.Document == nil {
		return errors.New("'document' is missing")
	}
	if err := adv.Document.Validate(); err != nil {
		return fmt.Errorf("'document' is invalid: %w", err)
	}
	if adv.ProductTree != nil {
		if err := adv.ProductTree.Validate(); err != nil {
			return fmt.Errorf("'product_tree' is invalid: %w", err)
		}
	}
	if adv.Vulnerabilities != nil {
		if err := adv.Vulnerabilities.Validate(); err != nil {
			return fmt.Errorf("'vulnerabilities' is invalid: %w", err)
		}
	}
	return nil
}

// LoadAdvisory loads an advisory from a file.
func LoadAdvisory(fname string) (*Advisory, error) {
	f, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var advisory Advisory
	if err := misc.StrictJSONParse(f, &advisory); err != nil {
		return nil, err
	}
	if err := advisory.Validate(); err != nil {
		return nil, err
	}
	return &advisory, nil
}

// SaveAdvisory writes the JSON encoding of the given advisory to a
// file with the given name.
// It returns nil, otherwise an error.
func SaveAdvisory(adv *Advisory, fname string) error {
	var w io.WriteCloser
	f, err := os.Create(fname)
	if err != nil {
		return err
	}
	w = f

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	err = enc.Encode(adv)
	if e := w.Close(); err != nil {
		err = e
	}
	return err
}

// UnmarshalText implements the encoding.TextUnmarshaller interface.
func (bc *BranchCategory) UnmarshalText(data []byte) error {
	s, err := csafBranchCategoryPattern(data)
	if err == nil {
		*bc = BranchCategory(s)
	}
	return err
}

// UnmarshalText implements the encoding.TextUnmarshaller interface.
func (nc *NoteCategory) UnmarshalText(data []byte) error {
	s, err := csafNoteCategoryPattern(data)
	if err == nil {
		*nc = NoteCategory(s)
	}
	return err
}

// UnmarshalText implements the encoding.TextUnmarshaller interface.
func (rc *ReferenceCategory) UnmarshalText(data []byte) error {
	s, err := csafReferenceCategoryPattern(data)
	if err == nil {
		*rc = ReferenceCategory(s)
	}
	return err
}

// UnmarshalText implements the encoding.TextUnmarshaller interface.
func (ts *TrackingStatus) UnmarshalText(data []byte) error {
	s, err := csafTrackingStatusPattern(data)
	if err == nil {
		*ts = TrackingStatus(s)
	}
	return err
}

// UnmarshalText implements the encoding.TextUnmarshaller interface.
func (rc *RelationshipCategory) UnmarshalText(data []byte) error {
	s, err := csafRelationshipCategoryPattern(data)
	if err == nil {
		*rc = RelationshipCategory(s)
	}
	return err
}

// UnmarshalText implements the encoding.TextUnmarshaller interface.
func (fl *FlagLabel) UnmarshalText(data []byte) error {
	s, err := csafFlagLabelPattern(data)
	if err == nil {
		*fl = FlagLabel(s)
	}
	return err
}

// UnmarshalText implements the encoding.TextUnmarshaller interface.
func (ip *InvolvementParty) UnmarshalText(data []byte) error {
	s, err := csafInvolvementPartyPattern(data)
	if err == nil {
		*ip = InvolvementParty(s)
	}
	return err
}

// UnmarshalText implements the encoding.TextUnmarshaller interface.
func (is *InvolvementStatus) UnmarshalText(data []byte) error {
	s, err := csafInvolvementStatusPattern(data)
	if err == nil {
		*is = InvolvementStatus(s)
	}
	return err
}

// UnmarshalText implements the encoding.TextUnmarshaller interface.
func (rc *RemediationCategory) UnmarshalText(data []byte) error {
	s, err := csafRemediationCategoryPattern(data)
	if err == nil {
		*rc = RemediationCategory(s)
	}
	return err
}

// UnmarshalText implements the encoding.TextUnmarshaller interface.
func (rrc *RestartRequiredCategory) UnmarshalText(data []byte) error {
	s, err := csafRestartRequiredCategoryPattern(data)
	if err == nil {
		*rrc = RestartRequiredCategory(s)
	}
	return err
}

// UnmarshalText implements the encoding.TextUnmarshaller interface.
func (tc *ThreatCategory) UnmarshalText(data []byte) error {
	s, err := csafThreatCategoryPattern(data)
	if err == nil {
		*tc = ThreatCategory(s)
	}
	return err
}

// UnmarshalText implements the encoding.TextUnmarshaller interface.
func (cpe *CPE) UnmarshalText(data []byte) error {
	s, err := cpePattern(data)
	if err == nil {
		*cpe = CPE(s)
	}
	return err
}

// UnmarshalText implements the encoding.TextUnmarshaller interface.
func (fhv *FileHashValue) UnmarshalText(data []byte) error {
	s, err := fileHashValuePattern(data)
	if err == nil {
		*fhv = FileHashValue(s)
	}
	return err
}

// UnmarshalText implements the encoding.TextUnmarshaller interface.
func (p *PURL) UnmarshalText(data []byte) error {
	s, err := pURLPattern(data)
	if err == nil {
		*p = PURL(s)
	}
	return err
}

// UnmarshalText implements the encoding.TextUnmarshaller interface.
func (l *Lang) UnmarshalText(data []byte) error {
	s, err := langPattern(data)
	if err == nil {
		*l = Lang(s)
	}
	return err
}

// UnmarshalText implements the encoding.TextUnmarshaller interface.
func (v *RevisionNumber) UnmarshalText(data []byte) error {
	s, err := versionPattern(data)
	if err == nil {
		*v = RevisionNumber(s)
	}
	return err
}

// UnmarshalText implements the encoding.TextUnmarshaller interface.
func (dc *DocumentCategory) UnmarshalText(data []byte) error {
	s, err := documentCategoryPattern(data)
	if err == nil {
		*dc = DocumentCategory(s)
	}
	return err
}

// UnmarshalText implements the encoding.TextUnmarshaller interface.
func (cv *Version) UnmarshalText(data []byte) error {
	s, err := csafVersionPattern(data)
	if err == nil {
		*cv = Version(s)
	}
	return err
}

// UnmarshalText implements the encoding.TextUnmarshaller interface.
func (ti *TrackingID) UnmarshalText(data []byte) error {
	s, err := trackingIDPattern(data)
	if err == nil {
		*ti = TrackingID(s)
	}
	return err
}

// UnmarshalText implements the encoding.TextUnmarshaller interface.
func (cve *CVE) UnmarshalText(data []byte) error {
	s, err := cvePattern(data)
	if err == nil {
		*cve = CVE(s)
	}
	return err
}

// UnmarshalText implements the encoding.TextUnmarshaller interface.
func (wi *WeaknessID) UnmarshalText(data []byte) error {
	s, err := weaknessIDPattern(data)
	if err == nil {
		*wi = WeaknessID(s)
	}
	return err
}

// UnmarshalText implements the encoding.TextUnmarshaller interface.
func (cv *CVSSVersion2) UnmarshalText(data []byte) error {
	s, err := cvssVersion2Pattern(data)
	if err == nil {
		*cv = CVSSVersion2(s)
	}
	return err
}

// UnmarshalText implements the encoding.TextUnmarshaller interface.
func (cvs *CVSS2VectorString) UnmarshalText(data []byte) error {
	s, err := cvss2VectorStringPattern(data)
	if err == nil {
		*cvs = CVSS2VectorString(s)
	}
	return err
}

// UnmarshalText implements the encoding.TextUnmarshaller interface.
func (cv *CVSSVersion3) UnmarshalText(data []byte) error {
	s, err := cvss3VersionPattern(data)
	if err == nil {
		*cv = CVSSVersion3(s)
	}
	return err
}

// UnmarshalText implements the encoding.TextUnmarshaller interface.
func (cvs *CVSS3VectorString) UnmarshalText(data []byte) error {
	s, err := cvss3VectorStringPattern(data)
	if err == nil {
		*cvs = CVSS3VectorString(s)
	}
	return err
}
