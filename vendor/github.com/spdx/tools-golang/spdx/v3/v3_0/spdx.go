package v3_0

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spdx/tools-golang/spdx/v3/internal"
	"github.com/spdx/tools-golang/spdx/v3/internal/ld"
)

// SPDX 3 model and serialization code is generated from internal/generate/main.go
// To regenerate all models, run: make generate

const (
	Version     = "3.0.1" // TODO is there a way to ascertain this version from generated code programmatically?
	NOASSERTION = "NOASSERTION"
)

type Document struct {
	SpdxDocument
	LDContext ld.Context
}

func (d *Document) UnmarshalJSON(data []byte) error {
	if d.LDContext == nil {
		d.LDContext = context()
	}
	err := d.FromJSON(bytes.NewReader(data))
	if err != nil {
		return err
	}
	return nil
}

func (d *Document) MarshalJSON() ([]byte, error) {
	if d.LDContext == nil {
		d.LDContext = context()
	}
	buf := bytes.Buffer{}
	err := d.Write(&buf)
	return buf.Bytes(), err
}

func (d *Document) Write(w io.Writer) error {
	return d.ToJSON(w)
}

func NewDocument(conformance ProfileIdentifierType, documentName string, createdBy AnyAgent, createdUsing AnyTool) *Document {
	if createdBy == nil {
		createdBy = &SoftwareAgent{
			Comment: "Created with github.com/spdx/tools-golang",
			Name:    "tools-golang",
		}
	}
	ci := &CreationInfo{
		SpecVersion:  Version,
		Created:      time.Now(),
		CreatedBy:    notNil(AgentList{createdBy}),
		CreatedUsing: notNil(ToolList{createdUsing}),
	}
	id := ""
	name := documentName
	if internal.IsURI(name) {
		id = name
		name = ""
	}
	return &Document{
		SpdxDocument: SpdxDocument{
			ID:                  id,
			Name:                name,
			CreationInfo:        ci,
			ProfileConformances: conformanceFrom(conformance),
		},
		LDContext: context(),
	}
}

func conformanceFrom(conformance ProfileIdentifierType) []ProfileIdentifierType {
	out := []ProfileIdentifierType{ProfileIdentifierType_Core}
	switch conformance {
	case ProfileIdentifierType_Core:
	case ProfileIdentifierType_Software:
		out = append(out, conformance)
	case ProfileIdentifierType_Ai:
		out = append(out, ProfileIdentifierType_Software, conformance)
	case ProfileIdentifierType_Dataset:
		out = append(out, ProfileIdentifierType_Software, ProfileIdentifierType_Ai, conformance)
	}
	return out
}

// Validate will validate the full SPDX document, optionally applying the same pre-processing that ToJSON performs to fill
// in missing CreationInfo and other data
func (d *Document) Validate(preProcess bool) error {
	if preProcess {
		// do all
		_ = d.ToJSON(io.Discard)
	}
	return ld.ValidateGraph(d.SpdxDocument)
}

// ToJSON first processes the document by:
//   - setting each Element's CreationInfo property to the SpdxDocument's CreationInfo if nil
//   - collecting every ElementCollection's object graph Element references to its Elements slice
//   - filling known required fields with NOASSERTION or similar left empty by conversion from 2.3
//
// ... and after this initial processing, outputs the document as compact JSON LD,
// including accounting for empty IDs by outputting blank node spdxId values
func (d *Document) ToJSON(writer io.Writer) error {
	// all element collections need to have all contained elements in the Elements property
	_ = ld.VisitObjectGraph(&d.SpdxDocument, func(path []any, e AnyElement) error {
		// all elements need to have creationInfo set
		if e.GetCreationInfo() == nil {
			e.SetCreationInfo(d.CreationInfo)
		}
		switch e := e.(type) {
		case AnyElementCollection:
			// collect all unique elements in the collection graph and set in the Elements property
			e.SetElements(collectAllElements(e))
		case AnyLicense:
			// licenses are frequently missing required fields: during 2.3 conversion we have to convert license expressions to a full object graph,
			// not LicenseExpression because this is broken in 3.0 and only fixed in 3.1, which is not released. In 3.0 it cannot support
			// CustomLicenses, which many expressions have, so we must use the expanded licensing model to properly capture this, but these do not
			// have _text_, so we help users by filling these with NOASSERTION
			if e.GetText() == "" {
				e.SetText(NOASSERTION)
			}
		}
		return nil
	})

	// The Elements list should not be serialized - the graph of the SpdxDocument includes all other properties, such as RootElements
	elements := d.Elements
	defer func() { d.Elements = elements }()
	d.Elements = nil

	if d.LDContext == nil {
		d.LDContext = context()
	}

	// our default behavior is to ensure a URI for the document prefix, defaulting to a sub-URI of the document ID
	documentPrefix := d.ID
	if !internal.IsURI(documentPrefix) {
		name := d.ID
		if name == "" {
			name = d.Name
		}
		documentPrefix = internal.NewDocumentID(name)
		if d.ID == "" {
			d.ID = documentPrefix
		}
	}

	namespaceMap := map[string]string{}
	defaultPrefix := internal.DefaultSpdxNamespace
	for _, mapEntry := range d.NamespaceMaps {
		namespaceMap[string(mapEntry.GetNamespace())] = mapEntry.GetPrefix()
		if strings.HasPrefix(string(mapEntry.GetNamespace()), documentPrefix) {
			// if the user has provided an ID and included a reference in the namespace map, we will just use the namespace prefix
			documentPrefix = mapEntry.GetPrefix()
			break
		}
		// if we need to use the default prefix, avoid clashes with existing namespace map entries
		for strings.HasPrefix(mapEntry.GetPrefix(), defaultPrefix) {
			defaultPrefix += "2"
		}
	}

	// If the prefix is still a URI, we add a namespace map for the default prefix to refer to this document
	if internal.IsURI(documentPrefix) {
		// at this point we have a URI, need to ensure a separator character so expansion makes sense
		if !strings.HasSuffix(documentPrefix, "/") && !strings.HasSuffix(documentPrefix, internal.DefaultSpdxNamespaceSeparator) {
			documentPrefix += internal.DefaultSpdxNamespaceSeparator
		}
		ns := documentPrefix
		d.NamespaceMaps = append(d.NamespaceMaps, &NamespaceMap{
			Prefix:    defaultPrefix,
			Namespace: URI(ns),
		})
		namespaceMap[ns] = defaultPrefix
		documentPrefix = defaultPrefix // each element will have a unique URI based on the spdx document namespace
	}

	return internal.ToJSON("https://spdx.org/rdf/3.0.1/spdx-context.jsonld", d.LDContext, &d.SpdxDocument, internal.PrefixedIdGenerator(documentPrefix, namespaceMap), writer)
}

func (d *Document) FromJSON(reader io.Reader) error {
	if d.LDContext == nil {
		d.LDContext = context()
	}
	graph, err := d.LDContext.FromJSON(reader)
	if err != nil {
		return err
	}
	for _, e := range graph {
		if doc, ok := e.(*SpdxDocument); ok {
			d.SpdxDocument = *doc

			var allElements []AnyElement
			for _, o := range graph {
				// collect all graph elements except SpdxDocument itself
				if el, ok := o.(AnyElement); ok && el != doc {
					allElements = append(allElements, el)
				}
			}
			d.Elements = allElements

			return nil
		}
	}
	return fmt.Errorf("no SPDX document found")
}

var _ interface {
	json.Marshaler
	json.Unmarshaler
} = (*Document)(nil)
