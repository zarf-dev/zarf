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

import "fmt"

// copyAndConvert returns a converted copy of the BOM, adhering to a given SpecVersion.
func (b BOM) copyAndConvert(specVersion SpecVersion) (*BOM, error) {
	var bomCopy BOM
	err := b.copy(&bomCopy)
	if err != nil {
		return nil, fmt.Errorf("failed to copy bom: %w", err)
	}

	bomCopy.convert(specVersion)
	return &bomCopy, nil
}

// convert modifies the BOM such that it adheres to a given SpecVersion.
func (b *BOM) convert(specVersion SpecVersion) {
	if specVersion < SpecVersion1_1 {
		b.SerialNumber = ""
		b.ExternalReferences = nil
	}
	if specVersion < SpecVersion1_2 {
		b.Dependencies = nil
		b.Metadata = nil
		b.Services = nil
	}
	if specVersion < SpecVersion1_3 {
		b.Compositions = nil
	}
	if specVersion < SpecVersion1_4 {
		b.Vulnerabilities = nil
	}
	if specVersion < SpecVersion1_5 {
		b.Annotations = nil
		b.Formulation = nil
	}
	if specVersion < SpecVersion1_6 {
		b.Declarations = nil
		b.Definitions = nil
	}
	if specVersion < SpecVersion1_7 {
		b.Citations = nil
		if b.Definitions != nil {
			b.Definitions.Patents = nil
		}
	}

	if b.Dependencies != nil && specVersion < SpecVersion1_6 {
		for i := range *b.Dependencies {
			(*b.Dependencies)[i].Provides = nil
		}
	}

	if b.Metadata != nil {
		if specVersion < SpecVersion1_3 {
			b.Metadata.Licenses = nil
			b.Metadata.Properties = nil
		}
		if specVersion < SpecVersion1_5 {
			b.Metadata.Lifecycles = nil
		}

		if specVersion < SpecVersion1_6 {
			b.Metadata.Manufacturer = nil
		}

		if specVersion < SpecVersion1_7 {
			b.Metadata.DistributionConstraints = nil
		}

		recurseComponent(b.Metadata.Component, componentConverter(specVersion))
		convertLicenses(b.Metadata.Licenses, specVersion)
		convertTools(b.Metadata.Tools, specVersion)
		convertOrganizationalEntity(b.Metadata.Manufacture, specVersion)
		convertOrganizationalEntity(b.Metadata.Supplier, specVersion)

		if b.Metadata.Authors != nil {
			for i := range *b.Metadata.Authors {
				convertOrganizationalContact(&(*b.Metadata.Authors)[i], specVersion)
			}
		}
	}

	if b.Components != nil {
		for i := range *b.Components {
			recurseComponent(&(*b.Components)[i], componentConverter(specVersion))
		}
	}

	if b.Services != nil {
		for i := range *b.Services {
			recurseService(&(*b.Services)[i], serviceConverter(specVersion))
		}
	}

	if b.Vulnerabilities != nil {
		convertVulnerabilities(b.Vulnerabilities, specVersion)
	}

	if b.Compositions != nil {
		convertCompositions(b.Compositions, specVersion)
	}

	if b.ExternalReferences != nil {
		convertExternalReferences(b.ExternalReferences, specVersion)
	}

	if b.Annotations != nil {
		convertAnnotations(b.Annotations, specVersion)
	}

	b.SpecVersion = specVersion
	b.XMLNS = xmlNamespaces[specVersion]
	b.JSONSchema = jsonSchemas[specVersion]
}

// componentConverter modifies a Component such that it adheres to a given SpecVersion.
func componentConverter(specVersion SpecVersion) func(*Component) {
	return func(c *Component) {
		if specVersion < SpecVersion1_1 {
			c.BOMRef = ""
			c.ExternalReferences = nil
			if c.Modified == nil {
				c.Modified = Bool(false)
			}
			c.Pedigree = nil
		}

		if specVersion < SpecVersion1_2 {
			c.Author = ""
			c.MIMEType = ""
			if c.Pedigree != nil {
				c.Pedigree.Patches = nil
			}
			c.Supplier = nil
			c.SWID = nil
		}

		if specVersion < SpecVersion1_3 {
			c.Properties = nil
		}

		if specVersion < SpecVersion1_4 {
			c.ReleaseNotes = nil
			if c.Version == "" {
				c.Version = "0.0.0"
			}
		}

		if specVersion < SpecVersion1_5 {
			c.ModelCard = nil
			c.Data = nil
		}

		if specVersion < SpecVersion1_6 {
			c.SWHID = nil
			c.OmniborID = nil
			c.Manufacturer = nil
			c.Authors = nil
			c.Tags = nil
		}

		if specVersion < SpecVersion1_7 {
			c.IsExternal = nil
			c.PatentAssertions = nil
			c.VersionRange = ""
		}

		if !specVersion.supportsComponentType(c.Type) {
			c.Type = ComponentTypeApplication
		}

		convertExternalReferences(c.ExternalReferences, specVersion)
		convertHashes(c.Hashes, specVersion)
		convertLicenses(c.Licenses, specVersion)
		convertEvidence(c, specVersion)
		convertModelCard(c, specVersion)
		convertCryptoProperties(c.CryptoProperties, specVersion)

		if !specVersion.supportsScope(c.Scope) {
			c.Scope = ""
		}
	}
}

func convertEvidence(c *Component, specVersion SpecVersion) {
	if c.Evidence == nil {
		return
	}

	if specVersion < SpecVersion1_3 {
		c.Evidence = nil
		return
	}

	if specVersion < SpecVersion1_5 {
		c.Evidence.Identity = nil
		c.Evidence.Occurrences = nil
		c.Evidence.Callstack = nil
		return
	}

	if specVersion < SpecVersion1_6 {
		// Spec version 1.5 uses only one Identity.
		// cf. https://cyclonedx.org/docs/1.5/json/#components_items_evidence_identity
		if c.Evidence.Identity != nil && c.Evidence.Identity.Identities != nil {
			ids := *c.Evidence.Identity.Identities
			ids = ids[:1]
			c.Evidence.Identity = &EvidenceIdentityChoice{Identities: &ids}
		}
		if c.Evidence.Identity != nil && c.Evidence.Identity.Identities != nil {
			for i := range *c.Evidence.Identity.Identities {
				(*c.Evidence.Identity.Identities)[i].ConcludedValue = ""
			}
		}
		if c.Evidence.Occurrences != nil {
			for i := range *c.Evidence.Occurrences {
				occ := &(*c.Evidence.Occurrences)[i]

				occ.Line = nil
				occ.Offset = nil
				occ.Symbol = ""
				occ.AdditionalContext = ""
			}
		}
	}

	convertLicenses(c.Evidence.Licenses, specVersion)
}

func convertCompositions(comps *[]Composition, specVersion SpecVersion) {
	if comps == nil {
		return
	}

	for i := range *comps {
		comp := &(*comps)[i]
		if !specVersion.supportsCompositionAggregate(comp.Aggregate) {
			comp.Aggregate = CompositionAggregateUnknown
		}
	}
}

// convertDataClassifications modifies a DataClassification slice such that it adheres to a given SpecVersion.
func convertDataClassifications(dataClassifications *[]DataClassification, specVersion SpecVersion) {
	if dataClassifications == nil {
		return
	}

	// v1.6 introduced Name, Description, Governance, Source, and Destination fields
	if specVersion < SpecVersion1_6 {
		for i := range *dataClassifications {
			(*dataClassifications)[i].Name = ""
			(*dataClassifications)[i].Description = ""
			(*dataClassifications)[i].Governance = nil
			(*dataClassifications)[i].Source = nil
			(*dataClassifications)[i].Destination = nil
		}
	}
}

// convertExternalReferences modifies an ExternalReference slice such that it adheres to a given SpecVersion.
func convertExternalReferences(extRefs *[]ExternalReference, specVersion SpecVersion) {
	if extRefs == nil {
		return
	}

	for i := range *extRefs {
		extRef := &(*extRefs)[i]

		if !specVersion.supportsExternalReferenceType(extRef.Type) {
			extRef.Type = ERTypeOther
		}

		if specVersion < SpecVersion1_3 {
			extRef.Hashes = nil
		}

		if specVersion < SpecVersion1_7 {
			extRef.Properties = nil
		}
	}
}

// convertHashes modifies a Hash slice such that it adheres to a given SpecVersion.
// If after the conversion no valid hashes are left in the slice, it will be nilled.
func convertHashes(hashes *[]Hash, specVersion SpecVersion) {
	if hashes == nil {
		return
	}

	converted := make([]Hash, 0)
	for i := range *hashes {
		hash := (*hashes)[i]
		if specVersion.supportsHashAlgorithm(hash.Algorithm) {
			converted = append(converted, hash)
		}
	}

	if len(converted) == 0 {
		*hashes = nil
	} else {
		*hashes = converted
	}
}

// convertLicenses modifies a Licenses slice such that it adheres to a given SpecVersion.
// If after the conversion no valid licenses are left in the slice, it will be nilled.
func convertLicenses(licenses *Licenses, specVersion SpecVersion) {
	if licenses == nil {
		return
	}

	if specVersion < SpecVersion1_1 {
		converted := make(Licenses, 0)
		for i := range *licenses {
			choice := &(*licenses)[i]
			if choice.License != nil {
				if choice.License.ID == "" && choice.License.Name == "" {
					choice.License = nil
				} else {
					choice.License.Text = nil
					choice.License.URL = ""
				}
			}
			choice.Expression = ""
			if choice.License != nil {
				converted = append(converted, *choice)
			}
		}

		if len(converted) == 0 {
			*licenses = nil
		} else {
			*licenses = converted
		}
	}

	if specVersion < SpecVersion1_5 {
		for i := range *licenses {
			choice := &(*licenses)[i]
			if choice.License != nil {
				choice.License.BOMRef = ""
				choice.License.Licensing = nil
				choice.License.Properties = nil
			}
		}
	}

	if specVersion < SpecVersion1_6 {
		for i := range *licenses {
			choice := &(*licenses)[i]
			if choice.License == nil {
				continue
			}

			choice.License.Acknowledgement = ""

			if choice.License.Licensing == nil {
				continue
			}

			if choice.License.Licensing.Licensor != nil {
				convertOrganizationalEntity(choice.License.Licensing.Licensor.Organization, specVersion)
			}
			if choice.License.Licensing.Licensee != nil {
				convertOrganizationalEntity(choice.License.Licensing.Licensee.Organization, specVersion)
			}
			if choice.License.Licensing.Purchaser != nil {
				convertOrganizationalEntity(choice.License.Licensing.Purchaser.Organization, specVersion)
			}
		}
	}

	if specVersion < SpecVersion1_7 {
		for i := range *licenses {
			choice := &(*licenses)[i]
			choice.ExpressionDetails = nil
			choice.Licensing = nil
			choice.Properties = nil
		}
	}
}

func convertOrganizationalEntity(org *OrganizationalEntity, specVersion SpecVersion) {
	if org == nil {
		return
	}

	if specVersion < SpecVersion1_5 {
		org.BOMRef = ""

		if org.Contact != nil {
			for i := range *org.Contact {
				convertOrganizationalContact(&(*org.Contact)[i], specVersion)
			}
		}
	}

	if specVersion < SpecVersion1_6 {
		org.Address = nil
	}
}

func convertOrganizationalContact(c *OrganizationalContact, specVersion SpecVersion) {
	if c == nil {
		return
	}

	if specVersion < SpecVersion1_5 {
		c.BOMRef = ""
	}
}

func convertModelCard(c *Component, specVersion SpecVersion) {
	if c.ModelCard == nil {
		return
	}

	if specVersion < SpecVersion1_6 {
		if c.ModelCard.Considerations != nil {
			c.ModelCard.Considerations.EnvironmentalConsiderations = nil
		}
	}
}

func convertCryptoProperties(cp *CryptoProperties, specVersion SpecVersion) {
	if cp == nil {
		return
	}

	if cp.AlgorithmProperties != nil {
		if specVersion < SpecVersion1_7 {
			cp.AlgorithmProperties.AlgorithmFamily = ""
			cp.AlgorithmProperties.EllipticCurve = ""
		}
		if !specVersion.supportsCryptoPrimitive(cp.AlgorithmProperties.Primitive) {
			cp.AlgorithmProperties.Primitive = ""
		}
	}

	if cp.ProtocolProperties != nil && !specVersion.supportsCryptoProtocolType(cp.ProtocolProperties.Type) {
		cp.ProtocolProperties.Type = ""
	}

	if cp.CertificateProperties != nil {
		if specVersion < SpecVersion1_7 {
			cp.CertificateProperties.SerialNumber = ""
			cp.CertificateProperties.CertificateFileExtension = ""
			cp.CertificateProperties.Fingerprint = nil
			cp.CertificateProperties.CertificateState = nil
			cp.CertificateProperties.CertificateExtensions = nil
			cp.CertificateProperties.RelatedCryptographicAssets = nil
			cp.CertificateProperties.CreationDate = ""
			cp.CertificateProperties.ActivationDate = ""
			cp.CertificateProperties.DeactivationDate = ""
			cp.CertificateProperties.RevocationDate = ""
			cp.CertificateProperties.DestructionDate = ""
		}
	}

	if cp.RelatedCryptoMaterialProperties != nil {
		if specVersion < SpecVersion1_7 {
			cp.RelatedCryptoMaterialProperties.Fingerprint = nil
			cp.RelatedCryptoMaterialProperties.RelatedCryptographicAssets = nil
		}
	}

	if cp.ProtocolProperties != nil {
		if specVersion < SpecVersion1_7 {
			cp.ProtocolProperties.RelatedCryptographicAssets = nil
		}

		if cp.ProtocolProperties.CipherSuites != nil {
			for i := range *cp.ProtocolProperties.CipherSuites {
				cs := &(*cp.ProtocolProperties.CipherSuites)[i]
				if specVersion < SpecVersion1_7 {
					cs.TLSGroups = nil
					cs.TLSSignatureSchemes = nil
				}
			}
		}
	}
}

func convertVulnerabilities(vulns *[]Vulnerability, specVersion SpecVersion) {
	if vulns == nil {
		return
	}

	for i := range *vulns {
		vuln := &(*vulns)[i]

		convertTools(vuln.Tools, specVersion)

		if specVersion < SpecVersion1_5 {
			vuln.ProofOfConcept = nil
			vuln.Rejected = ""
			vuln.Workaround = ""
		}

		if specVersion < SpecVersion1_6 {
			if vuln.Credits != nil {
				if vuln.Credits.Organizations != nil {
					for i := range *vuln.Credits.Organizations {
						convertOrganizationalEntity(&(*vuln.Credits.Organizations)[i], specVersion)
					}
				}

				if vuln.Credits.Individuals != nil {
					for i := range *vuln.Credits.Individuals {
						convertOrganizationalContact(&(*vuln.Credits.Individuals)[i], specVersion)
					}
				}
			}
		}

		if vuln.Ratings != nil {
			for j := range *vuln.Ratings {
				rating := &(*vuln.Ratings)[j]
				if !specVersion.supportsScoringMethod(rating.Method) {
					rating.Method = ScoringMethodOther
				}
			}
		}
	}
}

func convertAnnotations(annotations *[]Annotation, specVersion SpecVersion) {
	if annotations == nil {
		return
	}

	if specVersion < SpecVersion1_6 {
		for i := range *annotations {
			ann := (*annotations)[i]

			if ann.Annotator == nil {
				continue
			}

			convertOrganizationalEntity(ann.Annotator.Organization, specVersion)
			recurseService(ann.Annotator.Service, serviceConverter(specVersion))
		}
	}
}

// serviceConverter modifies a Service such that it adheres to a given SpecVersion.
func serviceConverter(specVersion SpecVersion) func(*Service) {
	return func(s *Service) {
		if specVersion < SpecVersion1_3 {
			s.Properties = nil
		}

		if specVersion < SpecVersion1_4 {
			s.ReleaseNotes = nil
		}

		if specVersion < SpecVersion1_5 {
			s.TrustZone = ""
		}

		if specVersion < SpecVersion1_6 {
			convertDataClassifications(s.Data, specVersion)
		}

		if specVersion < SpecVersion1_7 {
			s.PatentAssertions = nil
		}

		convertOrganizationalEntity(s.Provider, specVersion)
		convertExternalReferences(s.ExternalReferences, specVersion)
	}
}

// convertTools modifies a ToolsChoice such that it adheres to a given SpecVersion.
func convertTools(tools *ToolsChoice, specVersion SpecVersion) {
	if tools == nil {
		return
	}

	if specVersion < SpecVersion1_5 {
		convertedTools := make([]Tool, 0)
		if tools.Components != nil {
			for i := range *tools.Components {
				tool := convertComponentToTool((*tools.Components)[i], specVersion)
				if tool != nil {
					convertedTools = append(convertedTools, *tool)
				}
			}
			tools.Components = nil
		}

		if tools.Services != nil {
			for i := range *tools.Services {
				tool := convertServiceToTool((*tools.Services)[i], specVersion)
				if tool != nil {
					convertedTools = append(convertedTools, *tool)
				}
			}
			tools.Services = nil
		}

		if len(convertedTools) > 0 {
			if tools.Tools == nil {
				tools.Tools = &convertedTools
			} else {
				*tools.Tools = append(*tools.Tools, convertedTools...)
			}
		}
	}

	if tools.Services != nil {
		for i := range *tools.Services {
			convertOrganizationalEntity((*tools.Services)[i].Provider, specVersion)
		}
	}

	if tools.Tools != nil {
		for i := range *tools.Tools {
			convertTool(&(*tools.Tools)[i], specVersion)
		}
	}
}

// convertTool modifies a Tool such that it adheres to a given SpecVersion.
func convertTool(tool *Tool, specVersion SpecVersion) {
	if tool == nil {
		return
	}

	if specVersion < SpecVersion1_4 {
		tool.ExternalReferences = nil
	}

	convertExternalReferences(tool.ExternalReferences, specVersion)
	convertHashes(tool.Hashes, specVersion)
}

// convertComponentToTool converts a Component to a Tool for use in ToolsChoice.Tools.
func convertComponentToTool(component Component, _ SpecVersion) *Tool {
	tool := Tool{
		Vendor:             component.Author,
		Name:               component.Name,
		Version:            component.Version,
		Hashes:             component.Hashes,
		ExternalReferences: component.ExternalReferences,
	}

	if component.Supplier != nil {
		// There is no perfect 1:1 mapping for the Vendor field, but Supplier comes closest.
		// https://github.com/CycloneDX/cyclonedx-go/issues/115#issuecomment-1688710539
		tool.Vendor = component.Supplier.Name
	}

	return &tool
}

// convertServiceToTool converts a Service to a Tool for use in ToolsChoice.Tools.
func convertServiceToTool(service Service, _ SpecVersion) *Tool {
	tool := Tool{
		Name:               service.Name,
		Version:            service.Version,
		ExternalReferences: service.ExternalReferences,
	}

	if service.Provider != nil {
		tool.Vendor = service.Provider.Name
	}

	return &tool
}

func recurseComponent(component *Component, f func(c *Component)) {
	if component == nil {
		return
	}

	f(component)

	if component.Components != nil {
		for i := range *component.Components {
			recurseComponent(&(*component.Components)[i], f)
		}
	}
	if component.Pedigree != nil {
		if component.Pedigree.Ancestors != nil {
			for i := range *component.Pedigree.Ancestors {
				recurseComponent(&(*component.Pedigree.Ancestors)[i], f)
			}
		}
		if component.Pedigree.Descendants != nil {
			for i := range *component.Pedigree.Descendants {
				recurseComponent(&(*component.Pedigree.Descendants)[i], f)
			}
		}
		if component.Pedigree.Variants != nil {
			for i := range *component.Pedigree.Variants {
				recurseComponent(&(*component.Pedigree.Variants)[i], f)
			}
		}
	}
}

func recurseService(service *Service, f func(s *Service)) {
	if service == nil {
		return
	}

	f(service)

	if service.Services != nil {
		for i := range *service.Services {
			recurseService(&(*service.Services)[i], f)
		}
	}
}

func (sv SpecVersion) supportsComponentType(cType ComponentType) bool {
	switch cType {
	case ComponentTypeApplication, ComponentTypeDevice, ComponentTypeFramework, ComponentTypeLibrary, ComponentTypeOS:
		return sv >= SpecVersion1_0
	case ComponentTypeFile:
		return sv >= SpecVersion1_1
	case ComponentTypeContainer, ComponentTypeFirmware:
		return sv >= SpecVersion1_2
	case ComponentTypeData, ComponentTypeDeviceDriver, ComponentTypeMachineLearningModel, ComponentTypePlatform:
		return sv >= SpecVersion1_5
	}

	return false
}

func (sv SpecVersion) supportsCompositionAggregate(ca CompositionAggregate) bool {
	switch ca {
	case CompositionAggregateIncompleteFirstPartyOpenSourceOnly, CompositionAggregateIncompleteFirstPartyProprietaryOnly,
		CompositionAggregateIncompleteThirdPartyOpenSourceOnly, CompositionAggregateIncompleteThirdPartyProprietaryOnly:
		return sv >= SpecVersion1_5
	}

	return sv >= SpecVersion1_3
}

func (sv SpecVersion) supportsExternalReferenceType(ert ExternalReferenceType) bool {
	switch ert {
	case ERTypeAdversaryModel,
		ERTypeAttestation,
		ERTypeCertificationReport,
		ERTypeCodifiedInfrastructure,
		ERTypeComponentAnalysisReport,
		ERTypeConfiguration,
		ERTypeDistributionIntake,
		ERTypeDynamicAnalysisReport,
		ERTypeEvidence,
		ERTypeExploitabilityStatement,
		ERTypeFormulation,
		ERTypeLog,
		ERTypeMaturityReport,
		ERTypeModelCard,
		ERTypePentestReport,
		ERTypeQualityMetrics,
		ERTypeRiskAssessment,
		ERTypeRuntimeAnalysisReport,
		ERTypeStaticAnalysisReport,
		ERTypeThreatModel,
		ERTypeVulnerabilityAssertion:
		return sv >= SpecVersion1_5
	case ERTypePatent, ERTypePatentFamily, ERTypePatentAssertion, ERTypeCitation:
		return sv >= SpecVersion1_7
	}

	return sv >= SpecVersion1_1
}

func (sv SpecVersion) supportsCryptoPrimitive(primitive CryptoPrimitive) bool {
	switch primitive {
	case CryptoPrimitiveKeyWrap:
		return sv >= SpecVersion1_7
	}
	return sv >= SpecVersion1_6
}

func (sv SpecVersion) supportsCryptoProtocolType(pt CryptoProtocolType) bool {
	switch pt {
	case CryptoProtocolTypeDTLS, CryptoProtocolTypeQUIC,
		CryptoProtocolTypeEAPAKA, CryptoProtocolTypeEAPAKAPrime,
		CryptoProtocolTypePRINS, CryptoProtocolType5GAKA:
		return sv >= SpecVersion1_7
	}
	return sv >= SpecVersion1_6
}

func (sv SpecVersion) supportsHashAlgorithm(algo HashAlgorithm) bool {
	switch algo {
	case HashAlgoMD5, HashAlgoSHA1, HashAlgoSHA256, HashAlgoSHA384, HashAlgoSHA512, HashAlgoSHA3_256, HashAlgoSHA3_512:
		return sv >= SpecVersion1_0
	case HashAlgoSHA3_384, HashAlgoBlake2b_256, HashAlgoBlake2b_384, HashAlgoBlake2b_512, HashAlgoBlake3:
		return sv >= SpecVersion1_2
	case HashAlgoStreebog256, HashAlgoStreebog512:
		return sv >= SpecVersion1_7
	}

	return false
}

func (sv SpecVersion) supportsScope(scope Scope) bool {
	switch scope {
	case ScopeRequired, ScopeOptional:
		return sv >= SpecVersion1_0
	case ScopeExcluded:
		return sv >= SpecVersion1_2
	}

	return false
}

func (sv SpecVersion) supportsScoringMethod(method ScoringMethod) bool {
	switch method {
	case ScoringMethodCVSSv2, ScoringMethodCVSSv3, ScoringMethodCVSSv31, ScoringMethodOWASP, ScoringMethodOther:
		return sv >= SpecVersion1_4
	case ScoringMethodCVSSv4, ScoringMethodSSVC:
		return sv >= SpecVersion1_5
	}

	return false
}
