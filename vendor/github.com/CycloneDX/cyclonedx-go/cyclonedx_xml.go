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
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
)

// bomReferenceXML is temporarily used for marshalling and unmarshalling
// BOMReference instances to and from XML.
type bomReferenceXML struct {
	Ref string `json:"-" xml:"ref,attr"`
}

func (b BOMReference) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	return e.EncodeElement(bomReferenceXML{Ref: string(b)}, start)
}

func (b *BOMReference) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	bXML := bomReferenceXML{}
	if err := d.DecodeElement(&bXML, &start); err != nil {
		return err
	}
	*b = BOMReference(bXML.Ref)
	return nil
}

func (c Copyright) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	return e.EncodeElement(c.Text, start)
}

func (c *Copyright) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var text string
	if err := d.DecodeElement(&text, &start); err != nil {
		return err
	}
	c.Text = text
	return nil
}

// dependencyXML is temporarily used for marshalling and unmarshalling
// Dependency instances to and from XML.
type dependencyXML struct {
	Ref          string           `xml:"ref,attr"`
	Dependencies *[]dependencyXML `xml:"dependency,omitempty"`
	Provides     *[]dependencyXML `xml:"provides,omitempty"`
}

func (d Dependency) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	xmlDep := dependencyXML{Ref: d.Ref}

	if d.Dependencies != nil && len(*d.Dependencies) > 0 {
		xmlDeps := make([]dependencyXML, len(*d.Dependencies))
		for i := range *d.Dependencies {
			xmlDeps[i] = dependencyXML{Ref: (*d.Dependencies)[i]}
		}
		xmlDep.Dependencies = &xmlDeps
	}

	if d.Provides != nil && len(*d.Provides) > 0 {
		xmlProvides := make([]dependencyXML, len(*d.Provides))
		for i := range *d.Provides {
			xmlProvides[i] = dependencyXML{Ref: (*d.Provides)[i]}
		}
		xmlDep.Provides = &xmlProvides
	}

	return e.EncodeElement(xmlDep, start)
}

func (d *Dependency) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	xmlDep := dependencyXML{}
	err := dec.DecodeElement(&xmlDep, &start)
	if err != nil {
		return err
	}

	dep := Dependency{Ref: xmlDep.Ref}
	if xmlDep.Dependencies != nil && len(*xmlDep.Dependencies) > 0 {
		deps := make([]string, len(*xmlDep.Dependencies))
		for i := range *xmlDep.Dependencies {
			deps[i] = (*xmlDep.Dependencies)[i].Ref
		}
		dep.Dependencies = &deps
	}

	if xmlDep.Provides != nil && len(*xmlDep.Provides) > 0 {
		provides := make([]string, len(*xmlDep.Provides))
		for i := range *xmlDep.Provides {
			provides[i] = (*xmlDep.Provides)[i].Ref
		}
		dep.Provides = &provides
	}

	*d = dep
	return nil
}

func (ev EnvironmentVariables) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if len(ev) == 0 {
		return nil
	}

	err := e.EncodeToken(start)
	if err != nil {
		return err
	}

	for _, choice := range ev {
		if choice.Property != nil && choice.Value != "" {
			return fmt.Errorf("either property or value must be set, but not both")
		}

		if choice.Property != nil {
			err = e.EncodeElement(choice.Property, xml.StartElement{Name: xml.Name{Local: "environmentVar"}})
			if err != nil {
				return err
			}
		} else if choice.Value != "" {
			err = e.EncodeElement(choice.Value, xml.StartElement{Name: xml.Name{Local: "value"}})
			if err != nil {
				return err
			}
		}
	}

	return e.EncodeToken(start.End())
}

func (ev *EnvironmentVariables) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	envVars := make([]EnvironmentVariableChoice, 0)

	for {
		token, err := d.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}

		switch tokenType := token.(type) {
		case xml.StartElement:
			switch tokenType.Name.Local {
			case "value":
				var value string
				err = d.DecodeElement(&value, &tokenType)
				if err != nil {
					return err
				}
				envVars = append(envVars, EnvironmentVariableChoice{Value: value})
			case "environmentVar":
				var property Property
				err = d.DecodeElement(&property, &tokenType)
				if err != nil {
					return err
				}
				envVars = append(envVars, EnvironmentVariableChoice{Property: &property})
			default:
				return fmt.Errorf("unknown element: %s", tokenType.Name.Local)
			}
		}
	}

	*ev = envVars
	return nil
}

func (l License) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if l.ID != "" && l.Name != "" {
		return fmt.Errorf("license must have either id or name, not both")
	}
	if l.ID == "" && l.Name == "" {
		return fmt.Errorf("license must have either id or name")
	}
	type Alias License
	return e.EncodeElement(Alias(l), start)
}

// licenseExpressionXML is used for marshaling/unmarshaling the simple <expression> XML element
// which carries the expression text as character data and acknowledgement/bom-ref as attributes.
type licenseExpressionXML struct {
	Expression      string `xml:",chardata"`
	Acknowledgement string `xml:"acknowledgement,attr,omitempty"`
	BOMRef          string `xml:"bom-ref,attr,omitempty"`
}

// licenseExpressionDetailedXML is used for marshaling/unmarshaling the <expression-detailed> element.
type licenseExpressionDetailedXML struct {
	Expression      string                       `xml:"expression,attr"`
	Acknowledgement string                       `xml:"acknowledgement,attr,omitempty"`
	BOMRef          string                       `xml:"bom-ref,attr,omitempty"`
	Details         []licenseExpressionDetailXML `xml:"details"`
}

type licenseExpressionDetailXML struct {
	LicenseIdentifier string        `xml:"license-identifier,attr"`
	Text              *AttachedText `xml:"text,omitempty"`
	URL               string        `xml:"url,omitempty"`
}

func (l Licenses) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if len(l) == 0 {
		return nil
	}

	if err := e.EncodeToken(start); err != nil {
		return err
	}

	for _, choice := range l {
		if choice.License != nil && choice.Expression != "" {
			return fmt.Errorf("either license or expression must be set, but not both")
		}

		if choice.License != nil {
			if err := e.EncodeElement(choice.License, xml.StartElement{Name: xml.Name{Local: "license"}}); err != nil {
				return err
			}
		} else if choice.Expression != "" {
			var ackStr string
			if choice.Acknowledgement != nil {
				ackStr = string(*choice.Acknowledgement)
			}
			if choice.ExpressionDetails != nil {
				// Use expression-detailed element when expressionDetails are present
				detailed := licenseExpressionDetailedXML{
					Expression:      choice.Expression,
					Acknowledgement: ackStr,
					BOMRef:          choice.BOMRef,
				}
				for _, d := range *choice.ExpressionDetails {
					detailed.Details = append(detailed.Details, licenseExpressionDetailXML{
						LicenseIdentifier: d.LicenseIdentifier,
						Text:              d.Text,
						URL:               d.URL,
					})
				}
				if err := e.EncodeElement(detailed, xml.StartElement{Name: xml.Name{Local: "expression-detailed"}}); err != nil {
					return err
				}
			} else {
				exprXML := licenseExpressionXML{
					Expression:      choice.Expression,
					Acknowledgement: ackStr,
					BOMRef:          choice.BOMRef,
				}
				if err := e.EncodeElement(exprXML, xml.StartElement{Name: xml.Name{Local: "expression"}}); err != nil {
					return err
				}
			}
		}
	}

	return e.EncodeToken(start.End())
}

func (l *Licenses) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	licenses := make([]LicenseChoice, 0)

	for {
		token, err := d.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}

		switch tokenType := token.(type) {
		case xml.StartElement:
			switch tokenType.Name.Local {
			case "expression":
				var exprXML licenseExpressionXML
				if err = d.DecodeElement(&exprXML, &tokenType); err != nil {
					return err
				}
				choice := LicenseChoice{
					Expression: exprXML.Expression,
					BOMRef:     exprXML.BOMRef,
				}
				if exprXML.Acknowledgement != "" {
					ack := LicenseAcknowledgement(exprXML.Acknowledgement)
					choice.Acknowledgement = &ack
				}
				licenses = append(licenses, choice)
			case "expression-detailed":
				var detailed licenseExpressionDetailedXML
				if err = d.DecodeElement(&detailed, &tokenType); err != nil {
					return err
				}
				choice := LicenseChoice{
					Expression: detailed.Expression,
					BOMRef:     detailed.BOMRef,
				}
				if detailed.Acknowledgement != "" {
					ack := LicenseAcknowledgement(detailed.Acknowledgement)
					choice.Acknowledgement = &ack
				}
				if len(detailed.Details) > 0 {
					details := make([]LicenseExpressionDetail, len(detailed.Details))
					for i, det := range detailed.Details {
						details[i] = LicenseExpressionDetail{
							LicenseIdentifier: det.LicenseIdentifier,
							Text:              det.Text,
							URL:               det.URL,
						}
					}
					choice.ExpressionDetails = &details
				}
				licenses = append(licenses, choice)
			case "license":
				var license License
				if err = d.DecodeElement(&license, &tokenType); err != nil {
					return err
				}
				licenses = append(licenses, LicenseChoice{License: &license})
			default:
				return fmt.Errorf("unknown element: %s", tokenType.Name.Local)
			}
		}
	}

	*l = licenses
	return nil
}

type mlDatasetChoiceRefXML struct {
	Ref string `json:"-" xml:"ref"`
}

type mlDatasetChoiceXML struct {
	Ref string `json:"-" xml:"ref"`
	ComponentData
}

func (dc MLDatasetChoice) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if dc.Ref != "" {
		return e.EncodeElement(mlDatasetChoiceRefXML{Ref: dc.Ref}, start)
	} else if dc.ComponentData != nil {
		return e.EncodeElement(dc.ComponentData, start)
	}

	return nil
}

func (dc *MLDatasetChoice) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var choice mlDatasetChoiceXML
	err := d.DecodeElement(&choice, &start)
	if err != nil {
		return err
	}

	if choice.Ref != "" {
		dc.Ref = choice.Ref
		return nil
	}

	if choice.ComponentData != (ComponentData{}) {
		dc.ComponentData = &choice.ComponentData
	}

	return nil
}

func (sv SpecVersion) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	return e.EncodeElement(sv.String(), start)
}

func (sv *SpecVersion) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var v string
	err := d.DecodeElement(&v, &start)
	if err != nil {
		return err
	}

	switch v {
	case SpecVersion1_0.String():
		*sv = SpecVersion1_0
	case SpecVersion1_1.String():
		*sv = SpecVersion1_1
	case SpecVersion1_2.String():
		*sv = SpecVersion1_2
	case SpecVersion1_3.String():
		*sv = SpecVersion1_3
	case SpecVersion1_4.String():
		*sv = SpecVersion1_4
	case SpecVersion1_5.String():
		*sv = SpecVersion1_5
	case SpecVersion1_6.String():
		*sv = SpecVersion1_6
	case SpecVersion1_7.String():
		*sv = SpecVersion1_7
	default:
		return ErrInvalidSpecVersion
	}

	return nil
}

type predefinedCertificateStateXML struct {
	State  CertificateStateType `xml:"state"`
	Reason string               `xml:"reason,omitempty"`
}

type customCertificateStateXML struct {
	Name        string `xml:"name"`
	Description string `xml:"description,omitempty"`
	Reason      string `xml:"reason,omitempty"`
}

func (cs CertificateState) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if cs.Predefined != nil && cs.Custom != nil {
		return fmt.Errorf("either a predefined or custom certificate state can be used, but not both")
	}
	if cs.Predefined != nil {
		return e.EncodeElement(predefinedCertificateStateXML{
			State:  cs.Predefined.State,
			Reason: cs.Predefined.Reason,
		}, start)
	}
	if cs.Custom != nil {
		return e.EncodeElement(customCertificateStateXML{
			Name:        cs.Custom.Name,
			Description: cs.Custom.Description,
			Reason:      cs.Custom.Reason,
		}, start)
	}
	return nil
}

func (cs *CertificateState) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var raw struct {
		State       string `xml:"state"`
		Name        string `xml:"name"`
		Description string `xml:"description"`
		Reason      string `xml:"reason"`
	}
	if err := d.DecodeElement(&raw, &start); err != nil {
		return err
	}
	if raw.State != "" {
		cs.Predefined = &PredefinedCertificateState{
			State:  CertificateStateType(raw.State),
			Reason: raw.Reason,
		}
	} else {
		cs.Custom = &CustomCertificateState{
			Name:        raw.Name,
			Description: raw.Description,
			Reason:      raw.Reason,
		}
	}
	return nil
}

type commonCertificateExtensionXML struct {
	Name  CertificateExtensionName `xml:"commonExtensionName"`
	Value string                   `xml:"commonExtensionValue,omitempty"`
}

type customCertificateExtensionXML struct {
	Name  string `xml:"customExtensionName"`
	Value string `xml:"customExtensionValue,omitempty"`
}

func (ce CertificateExtension) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if ce.Common != nil && ce.Custom != nil {
		return fmt.Errorf("either a common or custom certificate extension can be used, but not both")
	}
	if ce.Common != nil {
		return e.EncodeElement(commonCertificateExtensionXML{
			Name:  ce.Common.Name,
			Value: ce.Common.Value,
		}, start)
	}
	if ce.Custom != nil {
		return e.EncodeElement(customCertificateExtensionXML{
			Name:  ce.Custom.Name,
			Value: ce.Custom.Value,
		}, start)
	}
	return nil
}

func (ce *CertificateExtension) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var raw struct {
		CommonExtensionName  string `xml:"commonExtensionName"`
		CommonExtensionValue string `xml:"commonExtensionValue"`
		CustomExtensionName  string `xml:"customExtensionName"`
		CustomExtensionValue string `xml:"customExtensionValue"`
	}
	if err := d.DecodeElement(&raw, &start); err != nil {
		return err
	}
	if raw.CommonExtensionName != "" {
		ce.Common = &CommonCertificateExtension{
			Name:  CertificateExtensionName(raw.CommonExtensionName),
			Value: raw.CommonExtensionValue,
		}
	} else {
		ce.Custom = &CustomCertificateExtension{
			Name:  raw.CustomExtensionName,
			Value: raw.CustomExtensionValue,
		}
	}
	return nil
}

func (ac AsserterChoice) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if ac.Organization != nil && ac.Individual != nil {
		return fmt.Errorf("asserter can only be one of organization, individual, or ref")
	}
	if ac.Organization != nil {
		return e.EncodeElement(struct {
			Organization *OrganizationalEntity `xml:"organization"`
		}{Organization: ac.Organization}, start)
	}
	if ac.Individual != nil {
		return e.EncodeElement(struct {
			Individual *OrganizationalContact `xml:"contact"`
		}{Individual: ac.Individual}, start)
	}
	if ac.BOMRef != nil {
		return e.EncodeElement(struct {
			BOMRef *BOMReference `xml:"ref"`
		}{BOMRef: ac.BOMRef}, start)
	}
	return nil
}

func (ac *AsserterChoice) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var raw struct {
		Organization *OrganizationalEntity  `xml:"organization"`
		Individual   *OrganizationalContact `xml:"contact"`
		BOMRef       *BOMReference          `xml:"ref"`
	}
	if err := d.DecodeElement(&raw, &start); err != nil {
		return err
	}
	ac.Organization = raw.Organization
	ac.Individual = raw.Individual
	ac.BOMRef = raw.BOMRef
	return nil
}

func (v IKEv2Auth) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	type alias IKEv2Auth
	if v.BOMRef != "" {
		if err := e.EncodeToken(start); err != nil {
			return err
		}
		if err := e.EncodeToken(xml.CharData(string(v.BOMRef))); err != nil {
			return err
		}
		return e.EncodeToken(start.End())
	}
	return e.EncodeElement(alias(v), start)
}

func (v *IKEv2Auth) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var raw struct {
		Content   string `xml:",chardata"`
		Name      string `xml:"name"`
		Algorithm string `xml:"algorithm"`
	}
	if err := d.DecodeElement(&raw, &start); err != nil {
		return err
	}
	if content := strings.TrimSpace(raw.Content); content != "" && raw.Name == "" && raw.Algorithm == "" {
		v.BOMRef = BOMReference(content)
	} else {
		v.Name = raw.Name
		v.Algorithm = raw.Algorithm
	}
	return nil
}

func (v IKEv2Enc) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	type alias IKEv2Enc
	if v.BOMRef != "" {
		if err := e.EncodeToken(start); err != nil {
			return err
		}
		if err := e.EncodeToken(xml.CharData(string(v.BOMRef))); err != nil {
			return err
		}
		return e.EncodeToken(start.End())
	}
	return e.EncodeElement(alias(v), start)
}

func (v *IKEv2Enc) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var raw struct {
		Content   string `xml:",chardata"`
		Name      string `xml:"name"`
		KeyLength *int   `xml:"keyLength"`
		Algorithm string `xml:"algorithm"`
	}
	if err := d.DecodeElement(&raw, &start); err != nil {
		return err
	}
	if content := strings.TrimSpace(raw.Content); content != "" && raw.Name == "" && raw.Algorithm == "" && raw.KeyLength == nil {
		v.BOMRef = BOMReference(content)
	} else {
		v.Name = raw.Name
		v.KeyLength = raw.KeyLength
		v.Algorithm = raw.Algorithm
	}
	return nil
}

func (v IKEv2Integ) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	type alias IKEv2Integ
	if v.BOMRef != "" {
		if err := e.EncodeToken(start); err != nil {
			return err
		}
		if err := e.EncodeToken(xml.CharData(string(v.BOMRef))); err != nil {
			return err
		}
		return e.EncodeToken(start.End())
	}
	return e.EncodeElement(alias(v), start)
}

func (v *IKEv2Integ) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var raw struct {
		Content   string `xml:",chardata"`
		Name      string `xml:"name"`
		Algorithm string `xml:"algorithm"`
	}
	if err := d.DecodeElement(&raw, &start); err != nil {
		return err
	}
	if content := strings.TrimSpace(raw.Content); content != "" && raw.Name == "" && raw.Algorithm == "" {
		v.BOMRef = BOMReference(content)
	} else {
		v.Name = raw.Name
		v.Algorithm = raw.Algorithm
	}
	return nil
}

func (v IKEv2Ke) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	type alias IKEv2Ke
	if v.BOMRef != "" {
		if err := e.EncodeToken(start); err != nil {
			return err
		}
		if err := e.EncodeToken(xml.CharData(string(v.BOMRef))); err != nil {
			return err
		}
		return e.EncodeToken(start.End())
	}
	return e.EncodeElement(alias(v), start)
}

func (v *IKEv2Ke) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var raw struct {
		Content   string `xml:",chardata"`
		Group     *int   `xml:"group"`
		Algorithm string `xml:"algorithm"`
	}
	if err := d.DecodeElement(&raw, &start); err != nil {
		return err
	}
	if content := strings.TrimSpace(raw.Content); content != "" && raw.Algorithm == "" && raw.Group == nil {
		v.BOMRef = BOMReference(content)
	} else {
		v.Group = raw.Group
		v.Algorithm = raw.Algorithm
	}
	return nil
}

func (v IKEv2Prf) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	type alias IKEv2Prf
	if v.BOMRef != "" {
		if err := e.EncodeToken(start); err != nil {
			return err
		}
		if err := e.EncodeToken(xml.CharData(string(v.BOMRef))); err != nil {
			return err
		}
		return e.EncodeToken(start.End())
	}
	return e.EncodeElement(alias(v), start)
}

func (v *IKEv2Prf) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var raw struct {
		Content   string `xml:",chardata"`
		Name      string `xml:"name"`
		Algorithm string `xml:"algorithm"`
	}
	if err := d.DecodeElement(&raw, &start); err != nil {
		return err
	}
	if content := strings.TrimSpace(raw.Content); content != "" && raw.Name == "" && raw.Algorithm == "" {
		v.BOMRef = BOMReference(content)
	} else {
		v.Name = raw.Name
		v.Algorithm = raw.Algorithm
	}
	return nil
}

// toolsChoiceMarshalXML is a helper struct for marshalling ToolsChoice.
type toolsChoiceMarshalXML struct {
	LegacyTools *[]Tool      `json:"-" xml:"tool,omitempty"`
	Components  *[]Component `json:"-" xml:"components>component,omitempty"`
	Services    *[]Service   `json:"-" xml:"services>service,omitempty"`
}

// toolsChoiceUnmarshalXML is a helper struct for unmarshalling tools represented
// as components and / or services. It is intended to be used with the streaming XML API.
//
//	<components>   <-- cursor should be here when unmarshalling this!
//	  <component>
//	    <name>foo</name>
//	  </component>
//	</components>
type toolsChoiceUnmarshalXML struct {
	Components *[]Component `json:"-" xml:"component,omitempty"`
	Services   *[]Service   `json:"-" xml:"service,omitempty"`
}

func (tc ToolsChoice) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if tc.Tools != nil && (tc.Components != nil || tc.Services != nil) {
		return fmt.Errorf("either a list of tools, or an object holding components and services can be used, but not both")
	}

	if tc.Tools != nil {
		return e.EncodeElement(toolsChoiceMarshalXML{LegacyTools: tc.Tools}, start)
	}

	tools := toolsChoiceMarshalXML{
		Components: tc.Components,
		Services:   tc.Services,
	}
	if tools.Components != nil || tools.Services != nil {
		return e.EncodeElement(tools, start)
	}

	return nil
}

func (tc *ToolsChoice) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	var components []Component
	var services []Service
	legacyTools := make([]Tool, 0)

	for {
		token, err := d.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}

		switch tokenType := token.(type) {
		case xml.StartElement:
			switch tokenType.Name.Local {
			case "tool":
				var tool Tool
				if err = d.DecodeElement(&tool, &tokenType); err != nil {
					return err
				}
				legacyTools = append(legacyTools, tool)
			case "components":
				var foo toolsChoiceUnmarshalXML
				if err = d.DecodeElement(&foo, &tokenType); err != nil {
					return err
				}
				if foo.Components != nil {
					components = *foo.Components
				}
			case "services":
				var foo toolsChoiceUnmarshalXML
				if err = d.DecodeElement(&foo, &tokenType); err != nil {
					return err
				}
				if foo.Services != nil {
					services = *foo.Services
				}
			default:
				return fmt.Errorf("unknown element: %s", tokenType.Name.Local)
			}
		}
	}

	choice := ToolsChoice{}
	if len(legacyTools) > 0 && (len(components) > 0 || len(services) > 0) {
		return fmt.Errorf("either a list of tools, or an object holding components and services can be used, but not both")
	}
	if len(components) > 0 {
		choice.Components = &components
	}
	if len(services) > 0 {
		choice.Services = &services
	}
	if len(legacyTools) > 0 {
		choice.Tools = &legacyTools
	}

	if choice.Tools != nil || choice.Components != nil || choice.Services != nil {
		*tc = choice
	}

	return nil
}

// EvidenceMarshalXML is temporarily used for marshalling
// Evidence instances from XML.
type EvidenceMarshalXML struct {
	Identity    *[]EvidenceIdentity   `json:"-" xml:"identity,omitempty"`
	Occurrences *[]EvidenceOccurrence `json:"-" xml:"occurrences>occurrence,omitempty"`
	Callstack   *Callstack            `json:"-" xml:"callstack,omitempty"`
	Licenses    *Licenses             `json:"-" xml:"licenses,omitempty"`
	Copyright   *[]Copyright          `json:"-" xml:"copyright>text,omitempty"`
}

// EvidenceUnmarshalXML is temporarily used for unmarshalling
// Evidence instances from XML.
type EvidenceUnmarshalXML struct {
	Occurrences *[]EvidenceOccurrence `json:"-" xml:"occurrence,omitempty"`
	Copyright   *[]Copyright          `json:"-" xml:"text,omitempty"`
}

func (ev Evidence) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	evidenceXML := EvidenceMarshalXML{}
	empty := true
	if ev.Identity != nil {
		if ev.Identity.Identities != nil {
			evidenceXML.Identity = ev.Identity.Identities
		} else if ev.Identity.Identity != nil {
			evidenceXML.Identity = &[]EvidenceIdentity{*ev.Identity.Identity}
		}
		empty = false
	}
	if ev.Occurrences != nil {
		evidenceXML.Occurrences = ev.Occurrences
		empty = false
	}
	if ev.Callstack != nil {
		evidenceXML.Callstack = ev.Callstack
		empty = false
	}
	if ev.Licenses != nil {
		evidenceXML.Licenses = ev.Licenses
		empty = false
	}
	if ev.Copyright != nil {
		evidenceXML.Copyright = ev.Copyright
		empty = false
	}

	if !empty {
		return e.EncodeElement(evidenceXML, start)
	}

	return nil
}

func (ev *Evidence) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	var evidence Evidence
	var identifies []EvidenceIdentity

	for {
		token, err := d.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}

		switch tokenType := token.(type) {
		case xml.StartElement:
			switch tokenType.Name.Local {
			case "identity":
				var identity EvidenceIdentity
				if err = d.DecodeElement(&identity, &tokenType); err != nil {
					return err
				}
				identifies = append(identifies, identity)
			case "occurrences":
				var evidenceXml EvidenceUnmarshalXML
				if err = d.DecodeElement(&evidenceXml, &tokenType); err != nil {
					return err
				}
				if evidenceXml.Occurrences != nil {
					evidence.Occurrences = evidenceXml.Occurrences
				}
			case "callstack":
				var cs Callstack
				if err = d.DecodeElement(&cs, &tokenType); err != nil {
					return err
				}
				if cs.Frames != nil {
					evidence.Callstack = &cs
				}
			case "licenses":
				var licenses Licenses
				if err = d.DecodeElement(&licenses, &tokenType); err != nil {
					return err
				}
				if len(licenses) > 0 {
					evidence.Licenses = &licenses
				}
			case "copyright":
				var evidenceXml EvidenceUnmarshalXML
				if err = d.DecodeElement(&evidenceXml, &tokenType); err != nil {
					return err
				}
				if evidenceXml.Copyright != nil {
					evidence.Copyright = evidenceXml.Copyright
				}
			default:
				return fmt.Errorf("unknown element: %s", tokenType.Name.Local)
			}
		}
	}

	if len(identifies) > 0 {
		evidence.Identity = &EvidenceIdentityChoice{Identities: &identifies}
	}

	*ev = evidence
	return nil
}

// MarshalXML implements custom XML marshaling for DataClassification to support the v1.6 dataflow format
func (dc DataClassification) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name.Local = "dataflow"

	// Add name and description as attributes if present
	if dc.Name != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "name"}, Value: dc.Name})
	}
	if dc.Description != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "description"}, Value: dc.Description})
	}

	if err := e.EncodeToken(start); err != nil {
		return err
	}

	// Encode classification element with flow attribute and text content
	if dc.Classification != "" || dc.Flow != "" {
		classStart := xml.StartElement{Name: xml.Name{Local: "classification"}}
		if dc.Flow != "" {
			classStart.Attr = append(classStart.Attr, xml.Attr{Name: xml.Name{Local: "flow"}, Value: string(dc.Flow)})
		}
		if err := e.EncodeToken(classStart); err != nil {
			return err
		}
		if dc.Classification != "" {
			if err := e.EncodeToken(xml.CharData(dc.Classification)); err != nil {
				return err
			}
		}
		if err := e.EncodeToken(xml.EndElement{Name: classStart.Name}); err != nil {
			return err
		}
	}

	// Encode governance
	if dc.Governance != nil {
		govStart := xml.StartElement{Name: xml.Name{Local: "governance"}}
		if err := e.EncodeElement(dc.Governance, govStart); err != nil {
			return err
		}
	}

	// Encode source URLs
	if dc.Source != nil && len(*dc.Source) > 0 {
		sourceStart := xml.StartElement{Name: xml.Name{Local: "source"}}
		if err := e.EncodeToken(sourceStart); err != nil {
			return err
		}
		for _, url := range *dc.Source {
			urlStart := xml.StartElement{Name: xml.Name{Local: "url"}}
			if err := e.EncodeElement(url, urlStart); err != nil {
				return err
			}
		}
		if err := e.EncodeToken(xml.EndElement{Name: sourceStart.Name}); err != nil {
			return err
		}
	}

	// Encode destination URLs
	if dc.Destination != nil && len(*dc.Destination) > 0 {
		destStart := xml.StartElement{Name: xml.Name{Local: "destination"}}
		if err := e.EncodeToken(destStart); err != nil {
			return err
		}
		for _, url := range *dc.Destination {
			urlStart := xml.StartElement{Name: xml.Name{Local: "url"}}
			if err := e.EncodeElement(url, urlStart); err != nil {
				return err
			}
		}
		if err := e.EncodeToken(xml.EndElement{Name: destStart.Name}); err != nil {
			return err
		}
	}

	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// UnmarshalXML implements custom XML unmarshaling for DataClassification to support the v1.6 dataflow format
func (dc *DataClassification) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	// Parse name and description from attributes
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "name":
			dc.Name = attr.Value
		case "description":
			dc.Description = attr.Value
		}
	}

	// Parse child elements
	for {
		token, err := d.Token()
		if err != nil {
			return err
		}

		switch el := token.(type) {
		case xml.StartElement:
			switch el.Name.Local {
			case "classification":
				// Parse flow attribute
				for _, attr := range el.Attr {
					if attr.Name.Local == "flow" {
						dc.Flow = DataFlow(attr.Value)
					}
				}
				// Parse classification text
				var content string
				if err := d.DecodeElement(&content, &el); err != nil {
					return err
				}
				dc.Classification = content

			case "governance":
				var gov DataGovernance
				if err := d.DecodeElement(&gov, &el); err != nil {
					return err
				}
				dc.Governance = &gov

			case "source":
				var urls []string
				for {
					token, err := d.Token()
					if err != nil {
						return err
					}
					if end, ok := token.(xml.EndElement); ok && end.Name.Local == "source" {
						break
					}
					if urlEl, ok := token.(xml.StartElement); ok && urlEl.Name.Local == "url" {
						var url string
						if err := d.DecodeElement(&url, &urlEl); err != nil {
							return err
						}
						urls = append(urls, url)
					}
				}
				if len(urls) > 0 {
					dc.Source = &urls
				}

			case "destination":
				var urls []string
				for {
					token, err := d.Token()
					if err != nil {
						return err
					}
					if end, ok := token.(xml.EndElement); ok && end.Name.Local == "destination" {
						break
					}
					if urlEl, ok := token.(xml.StartElement); ok && urlEl.Name.Local == "url" {
						var url string
						if err := d.DecodeElement(&url, &urlEl); err != nil {
							return err
						}
						urls = append(urls, url)
					}
				}
				if len(urls) > 0 {
					dc.Destination = &urls
				}
			}

		case xml.EndElement:
			if el.Name.Local == "dataflow" {
				return nil
			}
		}
	}
}

func (d Definitions) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if err := e.EncodeToken(start); err != nil {
		return err
	}

	if d.Standards != nil {
		standardsStart := xml.StartElement{Name: xml.Name{Local: "standards"}}
		if err := e.EncodeToken(standardsStart); err != nil {
			return err
		}
		for _, s := range *d.Standards {
			if err := e.EncodeElement(s, xml.StartElement{Name: xml.Name{Local: "standard"}}); err != nil {
				return err
			}
		}
		if err := e.EncodeToken(standardsStart.End()); err != nil {
			return err
		}
	}

	if d.Patents != nil {
		patentsStart := xml.StartElement{Name: xml.Name{Local: "patents"}}
		if err := e.EncodeToken(patentsStart); err != nil {
			return err
		}
		for _, choice := range *d.Patents {
			if choice.Patent != nil {
				if err := e.EncodeElement(choice.Patent, xml.StartElement{Name: xml.Name{Local: "patent"}}); err != nil {
					return err
				}
			} else if choice.PatentFamily != nil {
				if err := e.EncodeElement(choice.PatentFamily, xml.StartElement{Name: xml.Name{Local: "patentFamily"}}); err != nil {
					return err
				}
			}
		}
		if err := e.EncodeToken(patentsStart.End()); err != nil {
			return err
		}
	}

	return e.EncodeToken(start.End())
}

func (d *Definitions) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	for {
		token, err := dec.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}

		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "standards":
				var standards []StandardDefinition
				for {
					innerToken, err := dec.Token()
					if err != nil {
						if errors.Is(err, io.EOF) {
							break
						}
						return err
					}
					switch it := innerToken.(type) {
					case xml.StartElement:
						if it.Name.Local == "standard" {
							var s StandardDefinition
							if err = dec.DecodeElement(&s, &it); err != nil {
								return err
							}
							standards = append(standards, s)
						}
					case xml.EndElement:
						goto doneStandards
					}
				}
			doneStandards:
				if len(standards) > 0 {
					d.Standards = &standards
				}
			case "patents":
				var choices []PatentChoice
				for {
					innerToken, err := dec.Token()
					if err != nil {
						if errors.Is(err, io.EOF) {
							break
						}
						return err
					}
					switch it := innerToken.(type) {
					case xml.StartElement:
						switch it.Name.Local {
						case "patent":
							var p Patent
							if err = dec.DecodeElement(&p, &it); err != nil {
								return err
							}
							choices = append(choices, PatentChoice{Patent: &p})
						case "patentFamily":
							var pf PatentFamily
							if err = dec.DecodeElement(&pf, &it); err != nil {
								return err
							}
							choices = append(choices, PatentChoice{PatentFamily: &pf})
						}
					case xml.EndElement:
						goto donePatents
					}
				}
			donePatents:
				if len(choices) > 0 {
					d.Patents = &choices
				}
			default:
				if err = dec.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			return nil
		}
	}
	return nil
}

var xmlNamespaces = map[SpecVersion]string{
	SpecVersion1_0: "http://cyclonedx.org/schema/bom/1.0",
	SpecVersion1_1: "http://cyclonedx.org/schema/bom/1.1",
	SpecVersion1_2: "http://cyclonedx.org/schema/bom/1.2",
	SpecVersion1_3: "http://cyclonedx.org/schema/bom/1.3",
	SpecVersion1_4: "http://cyclonedx.org/schema/bom/1.4",
	SpecVersion1_5: "http://cyclonedx.org/schema/bom/1.5",
	SpecVersion1_6: "http://cyclonedx.org/schema/bom/1.6",
	SpecVersion1_7: "http://cyclonedx.org/schema/bom/1.7",
}
