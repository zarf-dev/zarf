package v3_0

import (
	"fmt"
	"os"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/spdx/tools-golang/spdx/v2/common"
	"github.com/spdx/tools-golang/spdx/v2/v2_3"
	"github.com/spdx/tools-golang/spdx/v3/internal"
	"github.com/spdx/tools-golang/spdx/v3/internal/ld"
)

func From_v2_3(doc v2_3.Document, d *Document) {
	c := newDocumentConverter(d)

	// The DocumentNamespace should have been a URI and is used as the document ID,
	// which must be a URI, so we convert to URI if it's not valid
	ns := doc.DocumentNamespace
	if !internal.IsURI(ns) {
		ns = internal.DefaultSpdxDocumentIDPrefix + ns
	}

	// namespace is used to prefix document IDs
	// in the document namespace, this results in logical urls when expanding IDs.
	// for example, if the v2 doc has http://example.org/abcd and a packageID is `Package-something-23`,
	// the v3 expanded IDs: http://example.org/abcd#Package-something-23
	if !strings.HasSuffix(ns, internal.DefaultSpdxNamespaceSeparator) {
		ns += internal.DefaultSpdxNamespaceSeparator
	}
	d.NamespaceMaps = NamespaceMapList{
		&NamespaceMap{
			// this will result in similar IDs output, e.g. SPDXRef-Package-something-23 -> spdx:Package-something-23
			Prefix:    internal.DefaultSpdxNamespace,
			Namespace: URI(ns),
		},
	}

	// set creationInfo of the converter so all created objects use the original
	c.creationInfo = c.convert23creationInfo(doc.CreationInfo)

	if d.CreationInfo == nil {
		d.CreationInfo = c.creationInfo
	}

	if len(d.ProfileConformances) == 0 {
		d.ProfileConformances = []ProfileIdentifierType{ProfileIdentifierType_Core, ProfileIdentifierType_Software}
	}

	// we have a doc.SPDXIdentifier, but this is always DOCUMENT in v2, referencing the document itself;
	// this will never be a valid URI, instead we want to use the DocumentNamespace, indicating the
	// document's URI in relation to all the contained elements
	d.ID = ns
	d.Comment = doc.DocumentComment
	d.Imports = list[ExternalMapList](c.convert23externalDocumentRef, doc.ExternalDocumentReferences...)
	d.Name = doc.DocumentName
	d.DataLicense = c.convert23licenseExpression(doc.DataLicense)

	var converted ElementList

	// other licenses may be referenced (with LicenseRef-... in expressions), so process these first to recreate the proper graph
	for _, l := range doc.OtherLicenses {
		converted = append(converted, c.convert23license(l))
	}

	for _, pkg := range doc.Packages {
		converted = append(converted, c.convert23package(pkg))
	}

	for _, file := range doc.Files {
		converted = append(converted, c.convert23file(file))
	}

	for _, a := range doc.Annotations {
		converted = append(converted, c.convert23annotation(a))
	}

	for _, s := range doc.Snippets {
		converted = append(converted, c.convert23snippet(s))
	}

	for _, rel := range doc.Relationships {
		c.convert23relationship(rel)
	}

	// add all converted elements to the sbom elements
	sbomElements := make(map[AnyElement]struct{}, len(c.sbom.Elements)+len(c.relationshipMap)+len(converted))
	for _, e := range c.sbom.Elements {
		sbomElements[e] = struct{}{}
	}
	for _, rels := range c.relationshipMap {
		for _, r := range rels {
			sbomElements[r] = struct{}{}
		}
	}
	for _, e := range converted {
		sbomElements[e] = struct{}{}
	}

	c.sbom.Elements = mapKeys(sbomElements)

	// ensure all elements are present in the document Elements list
	d.SpdxDocument.Elements = append(d.SpdxDocument.Elements, collectAllElements(&d.SpdxDocument)...)
}

func newDocumentConverter(d *Document) *documentConverter {
	if d.LDContext == nil {
		d.LDContext = context()
	}
	sbom := &SBOM{}
	d.RootElements = ElementList{sbom}
	c := &documentConverter{
		emailExtractor:  regexp.MustCompile(`(.*)\s*[(<]([^)>]+)[)>]$`),
		relationshipMap: map[any][]AnyRelationship{},
		sbom:            sbom,
		idMap: duplicateLower(map[string]any{
			"DOCUMENT":         sbom,
			"SPDXRef-DOCUMENT": sbom,
		}),
		lifecycleMap: duplicateLower(map[string]LifecycleScopeType{
			"BUILD_TOOL_OF":         LifecycleScopeType_Build,
			"BUILD_DEPENDENCY_OF":   LifecycleScopeType_Build,
			"DEV_DEPENDENCY_OF":     LifecycleScopeType_Development,
			"DEV_TOOL_OF":           LifecycleScopeType_Development,
			"RUNTIME_DEPENDENCY_OF": LifecycleScopeType_Runtime,
			"TEST_DEPENDENCY_OF":    LifecycleScopeType_Test,
			"TEST_TOOL_OF":          LifecycleScopeType_Test,
		}),
		inverseRelationshipMap: duplicateLower(map[string]RelationshipType{
			"DESCRIBED_BY":                RelationshipType_Describes,
			"BUILD_TOOL_OF":               RelationshipType_UsesTool,
			"CONTAINED_BY":                RelationshipType_Contains,
			"COPY_OF":                     RelationshipType_CopiedTo,
			"DATA_FILE_OF":                RelationshipType_HasDataFile,
			"DOCUMENTATION_OF":            RelationshipType_HasDocumentation,
			"DYNAMIC_LINK":                RelationshipType_HasDynamicLink,
			"EXPANDED_FROM_ARCHIVE":       RelationshipType_ExpandsTo,
			"FILE_ADDED":                  RelationshipType_HasAddedFile,
			"FILE_DELETED":                RelationshipType_HasDeletedFile,
			"GENERATED_FROM":              RelationshipType_Generates,
			"METAFILE_OF":                 RelationshipType_HasMetadata,
			"OPTIONAL_COMPONENT_OF":       RelationshipType_HasOptionalComponent,
			"PACKAGE_OF":                  RelationshipType_PackagedBy,
			"PATCH_APPLIED":               RelationshipType_PatchedBy,
			"PATCH_FOR":                   RelationshipType_PatchedBy,
			"AMENDS":                      RelationshipType_AmendedBy,
			"TEST_CASE_OF":                RelationshipType_HasTestCase,
			"PREREQUISITE_FOR":            RelationshipType_HasPrerequisite,
			"VARIANT_OF":                  RelationshipType_HasVariant,
			"BUILD_DEPENDENCY_OF":         RelationshipType_DependsOn,
			"DEPENDENCY_MANIFEST_OF":      RelationshipType_HasDependencyManifest,
			"DEPENDENCY_OF":               RelationshipType_DependsOn,
			"DEV_DEPENDENCY_OF":           RelationshipType_DependsOn,
			"DEV_TOOL_OF":                 RelationshipType_UsesTool,
			"EXAMPLE_OF":                  RelationshipType_HasExample,
			"OPTIONAL_DEPENDENCY_OF":      RelationshipType_HasOptionalDependency,
			"PROVIDED_DEPENDENCY_OF":      RelationshipType_HasProvidedDependency,
			"RUNTIME_DEPENDENCY_OF":       RelationshipType_DependsOn,
			"TEST_DEPENDENCY_OF":          RelationshipType_DependsOn,
			"TEST_OF":                     RelationshipType_HasTest,
			"TEST_TOOL_OF":                RelationshipType_UsesTool,
			"REQUIREMENT_DESCRIPTION_FOR": RelationshipType_HasRequirement,
			"SPECIFICATION_FOR":           RelationshipType_HasSpecification,
		}),
		relationshipTypeMap: duplicateLower(map[string]RelationshipType{
			"DESCRIBES":             RelationshipType_Describes,
			"ANCESTOR_OF":           RelationshipType_AncestorOf,
			"CONTAINS":              RelationshipType_Contains,
			"DESCENDANT_OF":         RelationshipType_DescendantOf,
			"DISTRIBUTION_ARTIFACT": RelationshipType_HasDistributionArtifact,
			"FILE_MODIFIED":         RelationshipType_ModifiedBy,
			"GENERATES":             RelationshipType_Generates,
			"OTHER":                 RelationshipType_Other,
			"STATIC_LINK":           RelationshipType_HasStaticLink,
			"HAS_PREREQUISITE":      RelationshipType_HasPrerequisite,
			"DEPENDS_ON":            RelationshipType_DependsOn,
		}),
		hashAlgorithmMap: duplicateLower(map[string]HashAlgorithm{
			"ADLER32":     HashAlgorithm_Adler32,
			"BLAKE2b_256": HashAlgorithm_Blake2b256,
			"BLAKE2b_384": HashAlgorithm_Blake2b384,
			"BLAKE2b_512": HashAlgorithm_Blake2b512,
			"BLAKE3":      HashAlgorithm_Blake3,
			"MD2":         HashAlgorithm_Md2,
			"MD4":         HashAlgorithm_Md4,
			"MD5":         HashAlgorithm_Md5,
			"MD6":         HashAlgorithm_Md6,
			"SHA1":        HashAlgorithm_Sha1,
			"SHA224":      HashAlgorithm_Sha224,
			"SHA256":      HashAlgorithm_Sha256,
			"SHA384":      HashAlgorithm_Sha384,
			"SHA3_256":    HashAlgorithm_Sha3_256,
			"SHA3_384":    HashAlgorithm_Sha3_384,
			"SHA3_512":    HashAlgorithm_Sha3_512,
			"SHA512":      HashAlgorithm_Sha512,
		}),
		annotationTypeMap: duplicateLower(map[string]AnnotationType{
			"OTHER":  AnnotationType_Other,
			"REVIEW": AnnotationType_Review,
		}),
		contentIdentifierTypeMap: duplicateLower(map[string]ContentIdentifierType{
			"gitoid": ContentIdentifierType_Gitoid,
			"swh":    ContentIdentifierType_Swhid,
		}),
		externalIdentifierTypeMap: duplicateLower(map[string]ExternalIdentifierType{
			"cpe22Type": ExternalIdentifierType_Cpe22,
			"cpe23Type": ExternalIdentifierType_Cpe23,
			"swid":      ExternalIdentifierType_Swid,
			"purl":      ExternalIdentifierType_PackageURL,
		}),
		externalRefTypeMap: duplicateLower(map[string]ExternalRefType{
			"maven-central": ExternalRefType_MavenCentral,
			"npm":           ExternalRefType_Npm,
			"nuget":         ExternalRefType_Nuget,
			"bower":         ExternalRefType_Bower,
			"advisory":      ExternalRefType_SecurityAdvisory,
			"fix":           ExternalRefType_SecurityFix,
			"url":           ExternalRefType_SecurityOther,
		}),
		primaryPurposeMap: duplicateLower(map[string]SoftwarePurpose{
			"APPLICATION":      SoftwarePurpose_Application,
			"BINARY":           SoftwarePurpose_Application,
			"ARCHIVE":          SoftwarePurpose_Archive,
			"CONTAINER":        SoftwarePurpose_Container,
			"DEVICE":           SoftwarePurpose_Device,
			"FILE":             SoftwarePurpose_File,
			"FIRMWARE":         SoftwarePurpose_Firmware,
			"FRAMEWORK":        SoftwarePurpose_Framework,
			"INSTALL":          SoftwarePurpose_Install,
			"LIBRARY":          SoftwarePurpose_Library,
			"OPERATING-SYSTEM": SoftwarePurpose_OperatingSystem,
			"OPERATING_SYSTEM": SoftwarePurpose_OperatingSystem,
			"OTHER":            SoftwarePurpose_Other,
			"SOURCE":           SoftwarePurpose_Source,
		}),
	}
	return c
}

type documentConverter struct {
	sbom                      *SBOM
	idMap                     map[string]any
	creationInfo              AnyCreationInfo
	relationshipMap           map[any][]AnyRelationship
	relationshipTypeMap       map[string]RelationshipType
	inverseRelationshipMap    map[string]RelationshipType
	lifecycleMap              map[string]LifecycleScopeType
	hashAlgorithmMap          map[string]HashAlgorithm
	annotationTypeMap         map[string]AnnotationType
	contentIdentifierTypeMap  map[string]ContentIdentifierType
	externalIdentifierTypeMap map[string]ExternalIdentifierType
	externalRefTypeMap        map[string]ExternalRefType
	primaryPurposeMap         map[string]SoftwarePurpose
	emailExtractor            *regexp.Regexp
	conversionErrors          []error
}

func (c *documentConverter) addRelationship(r AnyRelationship) {
	rels := c.relationshipMap[r.GetFrom()]
	for _, existing := range rels {
		if reflect.TypeOf(existing) != reflect.TypeOf(r) {
			// don't merge LifecycleScopedRelationship and Relationship
			continue
		}
		if existing.GetType() == r.GetType() {
			if r.GetComment() != existing.GetComment() {
				continue
			}
			if ls, ok := existing.(AnyLifecycleScopedRelationship); ok {
				if rs, ok := r.(AnyLifecycleScopedRelationship); ok {
					if ls.GetScope() != rs.GetScope() {
						continue
					}
				}
			}
			existing.SetTo(appendUnique(existing.GetTo(), r.GetTo()...))
			return
		}
	}
	c.relationshipMap[r.GetFrom()] = append(c.relationshipMap[r.GetFrom()], r)
}

func (c *documentConverter) convert23relationship(rel *v2_3.Relationship) {
	if rel == nil {
		return
	}
	from, _ := c.idMap[string(rel.RefA.ElementRefID)].(AnyElement)
	to, _ := c.idMap[string(rel.RefB.ElementRefID)].(AnyElement)
	if from == nil || to == nil {
		c.logDropped(rel)
		return
	}

	lifecycleScope, isLifecycle := c.lifecycleMap[rel.Relationship]

	typ, invert := c.convert23relationshipType(rel.Relationship)
	if invert {
		to, from = from, to
	}

	// SPDX 3 direct document DESCRIBES elements are in RootElement list, not relationships
	if from == c.sbom && typ == RelationshipType_Describes {
		c.sbom.RootElements = append(c.sbom.RootElements, to)
		return
	}

	if isLifecycle {
		c.addRelationship(&LifecycleScopedRelationship{
			Comment: rel.RelationshipComment,
			From:    from,
			Type:    typ,
			Scope:   lifecycleScope,
			To:      ElementList{to},
		})
	} else {
		c.addRelationship(&Relationship{
			Comment: rel.RelationshipComment,
			From:    from,
			Type:    typ,
			To:      ElementList{to},
		})
	}
}

func (c *documentConverter) convert23relationshipType(typ string) (RelationshipType, bool) {
	typ = strings.ToUpper(typ)
	out, ok := c.relationshipTypeMap[typ]
	if ok {
		return out, false
	}
	out, ok = c.inverseRelationshipMap[typ]
	if ok {
		return out, true
	}
	return RelationshipType{}, false
}

func (c *documentConverter) convert23creationInfo(info *v2_3.CreationInfo) AnyCreationInfo {
	if info == nil || len(info.Creators) == 0 {
		return nil
	}
	ci := &CreationInfo{
		Comment:      info.CreatorComment,
		Created:      c.convert23time(info.Created),
		CreatedBy:    list[AgentList](c.convert23creator, info.Creators...),
		CreatedUsing: list[ToolList](c.convert23tool, info.Creators...),
		SpecVersion:  Version, // specVersion is always the current version
	}

	if info.LicenseListVersion != "" {
		licenseListComment := fmt.Sprintf("LicenseListVersion: %v", info.LicenseListVersion)
		if ci.Comment == "" {
			ci.Comment = licenseListComment
		} else {
			ci.Comment = fmt.Sprintf("%v; %v", ci.Comment, licenseListComment)
		}
	}

	// update circular references, which will be set to nil by default
	for _, a := range ci.CreatedBy.Agents() {
		a.SetCreationInfo(ci)
	}
	for _, a := range ci.CreatedUsing.Tools() {
		a.SetCreationInfo(ci)
	}

	return ci
}

func (c *documentConverter) convert23tool(creator common.Creator) AnyTool {
	if strings.ToLower(creator.CreatorType) != "tool" {
		return nil
	}

	if creator.Creator == "" {
		c.logDropped(creator)
		return nil
	}

	return &Tool{
		CreationInfo: c.creationInfo,
		Name:         creator.Creator,
	}
}

func (c *documentConverter) convert23creator(creator common.Creator) AnyAgent {
	if strings.EqualFold(creator.CreatorType, "tool") {
		return nil // not applicable for creator
	}
	return c.convert23agent(creator.CreatorType, creator.Creator)
}

func (c *documentConverter) convert23originator(creator *common.Originator) AnyAgent {
	if creator == nil {
		return nil
	}
	return c.convert23agent(creator.OriginatorType, creator.Originator)
}

func (c *documentConverter) convert23supplier(creator *common.Supplier) AnyAgent {
	if creator == nil {
		return nil
	}
	return c.convert23agent(creator.SupplierType, creator.Supplier)
}

func (c *documentConverter) convert23annotator(creator *common.Annotator) AnyAgent {
	return c.convert23agent(creator.AnnotatorType, creator.Annotator)
}

func (c *documentConverter) convert23contributors(contributors ...string) AgentList {
	var agents AgentList
	for _, contributor := range contributors {
		parts := strings.Split(contributor, ":")
		if len(parts) > 1 && strings.EqualFold(parts[0], "person") {
			contributor = parts[1]
		}
		agent := c.convert23agent("person", contributor)
		if agent != nil {
			agents = append(agents, agent)
		}
	}
	return agents
}

func (c *documentConverter) convert23agent(typ, name string) AnyAgent {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	emailValue := ""
	match := c.emailExtractor.FindStringSubmatch(name)
	if len(match) > 2 {
		name = strings.TrimSpace(match[1])
		emailValue = strings.TrimSpace(match[2])
	}
	var out AnyAgent
	switch {
	case strings.EqualFold(typ, "person"):
		out = &Person{
			CreationInfo: c.creationInfo,
			Name:         name,
		}
	case strings.EqualFold(typ, "organization") || strings.EqualFold(typ, "org"):
		out = &Organization{
			CreationInfo: c.creationInfo,
			Name:         name,
		}
	case strings.EqualFold(typ, "tool"):
		out = &SoftwareAgent{
			CreationInfo: c.creationInfo,
			Name:         name,
		}
	default:
		c.logDropped(fmt.Sprintf("unknown agent type: %v with value: %v", typ, name))
	}
	if emailValue != "" {
		out.SetExternalIdentifiers(externalIdentifierListEmail(emailValue))
	}
	return out
}

func (c *documentConverter) convert23file(f *v2_3.File) AnyFile {
	if f == nil {
		return nil
	}

	out := &File{
		ID:               string(f.FileSPDXIdentifier),
		Comment:          f.FileComment,
		Name:             f.FileName,
		ExternalRefs:     nil,
		Summary:          "",
		VerifiedUsing:    list[IntegrityMethodList](c.convert23checksum, f.Checksums...),
		StandardNames:    nil,
		BuiltTime:        time.Time{},
		ReleaseTime:      time.Time{},
		SupportLevels:    nil,
		SuppliedBy:       nil,
		OriginatedBy:     c.convert23contributors(f.FileContributors...),
		ValidUntilTime:   time.Time{},
		AttributionTexts: f.FileAttributionTexts,
		CopyrightText:    f.FileCopyrightText,
		ContentType:      "",
		Kind:             FileKindType_File,
	}

	for _, typ := range f.FileTypes {
		purpose, ok := c.primaryPurposeMap[strings.ToUpper(typ)]
		if !ok || purpose == SoftwarePurpose_File {
			continue
		}
		emptyPurpose := SoftwarePurpose{}
		if out.PrimaryPurpose == emptyPurpose {
			out.PrimaryPurpose = purpose
		} else {
			out.AdditionalPurposes = append(out.AdditionalPurposes, purpose)
		}
	}

	c.idMap[string(f.FileSPDXIdentifier)] = out

	for _, s := range f.Snippets {
		v3 := c.convert23snippet(*s)
		c.addRelationship(&Relationship{
			Type: RelationshipType_Contains,
			From: out,
			To:   ElementList{v3},
		})
	}

	for _, a := range f.Annotations {
		v3 := c.convert23annotation(&a)
		v3.SetSubject(out)
		c.addRelationship(&Relationship{
			Type: RelationshipType_Describes,
			From: v3,
			To:   ElementList{out},
		})
	}

	if f.FileNotice != "" {
		v3 := &Annotation{
			Type:      AnnotationType_Other,
			Statement: f.FileNotice,
		}
		v3.SetSubject(out)
		c.addRelationship(&Relationship{
			Type: RelationshipType_Describes,
			From: v3,
			To:   ElementList{out},
		})
	}

	licenseComment := f.LicenseComments
	concluded := c.convert23licenseExpression(f.LicenseConcluded)
	if concluded != nil {
		concluded.SetComment(licenseComment)
		licenseComment = "" // only include in concluded if set
		c.addRelationship(&Relationship{
			Type: RelationshipType_HasConcludedLicense,
			From: out,
			To:   ElementList{concluded},
		})
	}

	for _, l := range f.LicenseInfoInFiles {
		d := c.convert23licenseExpression(l)
		if d != nil {
			d.SetComment(licenseComment)
			c.addRelationship(&Relationship{
				Type: RelationshipType_HasDeclaredLicense,
				From: out,
				To:   ElementList{d},
			})
		}
	}

	return out
}

func (c *documentConverter) convert23package(pkg *v2_3.Package) AnyPackage {
	if pkg == nil {
		return nil
	}

	var verificationCodes IntegrityMethodList

	if pkg.FilesAnalyzed {
		// verification code only valid if FilesAnalyzed
		verificationCode := c.convert23packageVerificationCode(pkg.PackageVerificationCode)
		if verificationCode != nil {
			verificationCodes = append(verificationCodes, verificationCode)
		}
	} else {
		c.logDropped(pkg.PackageVerificationCode)
	}

	for _, checksum := range pkg.PackageChecksums {
		ck := c.convert23checksum(checksum)
		if ck != nil {
			verificationCodes = append(verificationCodes, ck)
		}
	}

	id := string(pkg.PackageSPDXIdentifier)
	out := &Package{
		ID:                  id,
		Name:                pkg.PackageName,
		Summary:             pkg.PackageSummary,
		Comment:             pkg.PackageComment,
		Description:         pkg.PackageDescription,
		ExternalIdentifiers: list[ExternalIdentifierList](c.convert23externalIdentifier, pkg.PackageExternalReferences...),
		ExternalRefs:        list[ExternalRefList](c.convert23externalRef, pkg.PackageExternalReferences...),
		VerifiedUsing:       verificationCodes,
		BuiltTime:           c.convert23time(pkg.BuiltDate),
		OriginatedBy:        list[AgentList](c.convert23originator, pkg.PackageOriginator),
		ReleaseTime:         c.convert23time(pkg.ReleaseDate),
		SuppliedBy:          c.convert23supplier(pkg.PackageSupplier),
		ValidUntilTime:      c.convert23time(pkg.ValidUntilDate),
		AttributionTexts:    pkg.PackageAttributionTexts,
		CopyrightText:       pkg.PackageCopyrightText,
		PrimaryPurpose:      c.convert23purpose(pkg.PrimaryPackagePurpose),
		Version:             pkg.PackageVersion,
		DownloadLocation:    c.convert23uri(pkg.PackageDownloadLocation),
		HomePage:            c.convert23uri(pkg.PackageHomePage),
		PackageURL:          c.convert23packageUrl(pkg.PackageExternalReferences),
		SourceInfo:          pkg.PackageSourceInfo,
	}

	// move the first valid PURL to the PackageURL field
	for _, ident := range out.ExternalIdentifiers.ExternalIdentifiers() {
		if ident.GetType() == ExternalIdentifierType_PackageURL {
			if ident.GetComment() != "" {
				continue
			}
			purl := URI(ident.GetIdentifier())
			if purl.Validate() == nil {
				out.PackageURL = purl
				out.ExternalIdentifiers = slices.DeleteFunc(out.ExternalIdentifiers, func(identifier AnyExternalIdentifier) bool {
					return identifier.GetIdentifier() == string(purl)
				})
				break
			}
		}
	}

	c.idMap[id] = out

	if pkg.PackageLicenseComments != "" {
		if out.Comment == "" {
			out.Comment = pkg.PackageLicenseComments
		} else {
			// this appears to be the behavior from the Java tools:
			// https://github.com/spdx/Spdx-Java-Library/blob/e3640e27a423a5562c52bcc4075cce9ac35f433a/src/main/java/org/spdx/library/conversion/Spdx2to3Converter.java#L1233
			out.Comment += ";" + pkg.PackageLicenseComments
		}
	}

	for _, l := range pkg.PackageLicenseInfoFromFiles {
		d := c.convert23licenseExpression(l)
		if d != nil {
			c.addRelationship(&Relationship{
				Type: RelationshipType_HasDeclaredLicense,
				From: out,
				To:   ElementList{d},
			})
		}
	}
	d := c.convert23licenseExpression(pkg.PackageLicenseDeclared)
	if d != nil {
		c.addRelationship(&Relationship{
			Type: RelationshipType_HasDeclaredLicense,
			From: out,
			To:   ElementList{d},
		})
	}

	d = c.convert23licenseExpression(pkg.PackageLicenseConcluded)
	if d != nil {
		c.addRelationship(&Relationship{
			Type: RelationshipType_HasConcludedLicense,
			From: out,
			To:   ElementList{d},
		})
	}

	hasPackageFile := false
	for _, f := range pkg.Files {
		v3file := c.convert23file(f)
		if v3file == nil {
			continue
		}
		if v3file.GetName() == pkg.PackageName {
			hasPackageFile = true
		}
		c.addRelationship(&Relationship{
			Type: RelationshipType_Contains,
			From: out,
			To:   ElementList{v3file},
		})
	}

	if !hasPackageFile && pkg.PackageFileName != "" {
		v3file := &File{
			Name: pkg.PackageFileName,
		}
		c.addRelationship(&Relationship{
			Type: RelationshipType_HasDistributionArtifact,
			From: out,
			To:   ElementList{v3file},
		})
	}

	for _, a := range pkg.Annotations {
		v3 := c.convert23annotation(&a)
		v3.SetSubject(out)
		c.addRelationship(&Relationship{
			Type: RelationshipType_Describes,
			From: v3,
			To:   ElementList{out},
		})
	}

	return out
}

func (c *documentConverter) convert23time(date string) time.Time {
	if date == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, date)
	if err != nil {
		c.logDropped(err)
	}
	return t
}

func (c *documentConverter) convert23uri(uri string) ld.URI {
	out := ld.URI(uri)
	if out.Validate() == nil {
		return out
	}
	c.logDropped(out)
	return ""
}

func (c *documentConverter) logDropped(value any) {
	if err, ok := value.(error); ok {
		c.conversionErrors = append(c.conversionErrors, err)
	} else {
		c.conversionErrors = append(c.conversionErrors, fmt.Errorf("%v", value))
	}
	if value == nil || !internal.Debug {
		return
	}
	_, _ = fmt.Fprintf(os.Stderr, "dropped: %v", value)
}

func (c *documentConverter) convert23packageUrl(references []*v2_3.PackageExternalReference) ld.URI {
	for _, ref := range references {
		if ref != nil && ref.RefType == common.TypePackageManagerPURL {
			return c.convert23uri(ref.Locator)
		}
	}
	return ""
}

func (c *documentConverter) convert23purpose(purpose string) SoftwarePurpose {
	return c.primaryPurposeMap[strings.ToLower(purpose)]
}

func (c *documentConverter) convert23packageVerificationCode(v *common.PackageVerificationCode) AnyIntegrityMethod {
	if v == nil || v.Value == "" {
		c.logDropped(v)
		return nil
	}
	return &PackageVerificationCode{
		Algorithm:     HashAlgorithm_Sha1,
		HashValue:     v.Value,
		ExcludedFiles: v.ExcludedFiles,
	}
}

func (c *documentConverter) convert23externalIdentifier(r *v2_3.PackageExternalReference) AnyExternalIdentifier {
	if r == nil {
		return nil
	}
	typ, ok := c.externalIdentifierTypeMap[r.RefType]
	if !ok || r.Locator == "" {
		_, ok = c.externalRefTypeMap[r.RefType]
		if !ok {
			c.logDropped(r)
		}
		return nil
	}
	return &ExternalIdentifier{
		Comment:    r.ExternalRefComment,
		Type:       typ,
		Identifier: r.Locator,
	}
}

func (c *documentConverter) convert23externalRef(r *v2_3.PackageExternalReference) AnyExternalRef {
	if r == nil {
		return nil
	}
	typ, ok := c.externalRefTypeMap[r.RefType]
	if !ok || r.Locator == "" {
		// logged during convert23externalIdentifier
		return nil
	}
	return &ExternalRef{
		Comment:  r.ExternalRefComment,
		Type:     typ,
		Locators: []string{r.Locator},
	}
}

func (c *documentConverter) convert23license(l *v2_3.OtherLicense) AnyLicenseInfo {
	if l == nil {
		return nil
	}

	var seeAlso []ld.URI
	for _, ref := range l.LicenseCrossReferences {
		seeAlso = append(seeAlso, ld.URI(ref))
	}

	out := &CustomLicense{
		ID:       l.LicenseIdentifier,
		Name:     l.LicenseName,
		Comment:  l.LicenseComment,
		SeeAlsos: seeAlso,
		Text:     l.ExtractedText,
	}

	return c.resolveLicenseRefs(out)
}

func (c *documentConverter) convert23licenseExpression(licenseExpression string) AnyLicenseInfo {
	if licenseExpression == "" {
		return nil
	}

	parsedLicense, err := ParseLicenseExpression(licenseExpression)
	if err != nil {
		c.logDropped(err)
	}
	return c.resolveLicenseRefs(parsedLicense)
}

// resolveLicenseRefs walks a parsed license expression and replaces the
// placeholder CustomLicense references the parser produces for LicenseRef-*
// identifiers with the resolved CustomLicense instances registered in
// idMap by convert23license.
func (c *documentConverter) resolveLicenseRefs(license AnyLicenseInfo) AnyLicenseInfo {
	if isNil(license) { // avoid interface-to-nil values
		return nil
	}
	if license.GetID() != "" {
		if existing, _ := c.idMap[license.GetID()].(AnyLicenseInfo); existing != nil {
			return existing
		}
		c.idMap[license.GetID()] = license
	}
	switch l := license.(type) {
	case AnyConjunctiveLicenseSet:
		members := l.GetMembers()
		for i, m := range members {
			members[i] = c.resolveLicenseRefs(m)
		}
	case AnyDisjunctiveLicenseSet:
		members := l.GetMembers()
		for i, m := range members {
			members[i] = c.resolveLicenseRefs(m)
		}
	case AnyOrLaterOperator:
		if resolved, ok := c.resolveLicenseRefs(l.GetSubjectLicense()).(AnyLicense); ok {
			l.SetSubjectLicense(resolved)
		}
	case AnyWithAdditionOperator:
		if resolved, ok := c.resolveLicenseRefs(l.GetSubjectExtendableLicense()).(AnyExtendableLicense); ok {
			l.SetSubjectExtendableLicense(resolved)
		}
	}
	return license
}

func (c *documentConverter) convert23externalDocumentRef(r v2_3.ExternalDocumentRef) AnyExternalMap {
	if r.DocumentRefID == "" || r.URI == "" {
		c.logDropped(r)
		return nil
	}
	return &ExternalMap{
		ExternalSpdxID: ld.URI(r.DocumentRefID),
		LocationHint:   ld.URI(r.URI),
		VerifiedUsing:  list[IntegrityMethodList](c.convert23checksum, r.Checksum),
	}
}

func (c *documentConverter) convert23checksum(checksum common.Checksum) AnyIntegrityMethod {
	if checksum.Value == "" {
		c.logDropped(checksum)
		return nil
	}
	return &Hash{
		Value:     checksum.Value,
		Algorithm: c.hashAlgorithmMap[string(checksum.Algorithm)],
	}
}

func (c *documentConverter) convert23annotation(a *v2_3.Annotation) AnyAnnotation {
	if a == nil {
		return nil
	}

	typ, ok := c.annotationTypeMap[strings.ToUpper(a.AnnotationType)]
	if !ok {
		c.logDropped(a)
		return nil
	}

	out := &Annotation{
		CreationInfo: &CreationInfo{
			Created:   c.convert23time(a.AnnotationDate),
			CreatedBy: list[AgentList](c.convert23annotator, &a.Annotator),
		},
		// V3 Statement is "Commentary on an assertion that an annotator has made"
		Statement: a.AnnotationComment,
		Type:      typ,
	}

	to, _ := c.idMap[string(a.AnnotationSPDXIdentifier.ElementRefID)].(AnyElement)
	if to != nil {
		out.Subject = to
		c.addRelationship(&Relationship{
			Type: RelationshipType_Describes,
			From: out,
			To:   ElementList{to},
		})
	}
	return out
}

func (c *documentConverter) convert23snippet(s v2_3.Snippet) AnyElement {
	snippetFile, _ := c.idMap[string(s.SnippetFromFileSPDXIdentifier)].(AnyFile)

	concludedLicense := c.convert23licenseExpression(s.SnippetLicenseConcluded)

	var declaredLicenses LicenseInfoList
	for _, licenseInfo := range s.LicenseInfoInSnippet {
		d := c.convert23licenseExpression(licenseInfo)
		if d != nil {
			declaredLicenses = append(declaredLicenses, d)
		}
	}

	out := &Snippet{
		ID:               string(s.SnippetSPDXIdentifier),
		Comment:          s.SnippetComment,
		Name:             s.SnippetName,
		CopyrightText:    s.SnippetCopyrightText,
		AttributionTexts: s.SnippetAttributionTexts,
		FromFile:         snippetFile,
	}

	for _, r := range s.Ranges {
		// there are 2 spots that might hold references to the file, try to handle them both:
		f, _ := c.idMap[string(r.StartPointer.FileSPDXIdentifier)].(AnyFile)
		if f == nil {
			f = snippetFile
		}
		if snippetFile == nil {
			snippetFile = f
			out.FromFile = f
		}
		if r.StartPointer.Offset > 0 || r.EndPointer.Offset > 0 {
			out.ByteRange = &PositiveIntegerRange{
				BeginIntegerRange: ld.PositiveInt(r.StartPointer.Offset),
				EndIntegerRange:   ld.PositiveInt(r.EndPointer.Offset),
			}
		}
		if r.StartPointer.LineNumber > 0 || r.EndPointer.LineNumber > 0 {
			out.LineRange = &PositiveIntegerRange{
				BeginIntegerRange: ld.PositiveInt(r.StartPointer.LineNumber),
				EndIntegerRange:   ld.PositiveInt(r.EndPointer.LineNumber),
			}
		}
	}

	if concludedLicense != nil {
		concludedLicense.SetComment(s.SnippetLicenseComments)
		c.addRelationship(&Relationship{
			Type: RelationshipType_HasConcludedLicense,
			From: out,
			To:   ElementList{concludedLicense},
		})
	}

	for _, l := range declaredLicenses {
		if concludedLicense == nil {
			l.SetComment(s.SnippetLicenseComments)
		}
		c.addRelationship(&Relationship{
			Type: RelationshipType_HasDeclaredLicense,
			From: out,
			To:   ElementList{l},
		})
	}

	c.idMap[string(s.SnippetSPDXIdentifier)] = out

	return out
}

func appendUnique[T comparable](existing []T, adding ...T) []T {
	for _, add := range adding {
		if isNil(add) || slices.Contains(existing, add) {
			continue
		}
		existing = append(existing, add)
	}
	return existing
}

func list[ListType ~[]To, From, To any](convertFunc func(From) To, values ...From) ListType {
	var out ListType
	for _, v := range values {
		if isNil(v) {
			continue
		}
		o := convertFunc(v)
		if isNil(o) {
			continue
		}
		out = append(out, o)
	}
	return out
}

func isNil(o any) bool {
	v := reflect.ValueOf(o)
	return !v.IsValid() || v.IsZero()
}

func externalIdentifierListEmail(emailAddr string) ExternalIdentifierList {
	return ExternalIdentifierList{
		&ExternalIdentifier{
			Type:       ExternalIdentifierType_Email,
			Identifier: emailAddr,
		},
	}
}

func duplicateLower[T any](m map[string]T) map[string]T {
	for k, v := range m {
		m[strings.ToLower(k)] = v
	}
	return m
}
