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
	"encoding/json"
	"errors"
	"fmt"
)

func (ev EnvironmentVariableChoice) MarshalJSON() ([]byte, error) {
	if ev.Property != nil && *ev.Property != (Property{}) {
		return json.Marshal(ev.Property)
	} else if ev.Value != "" {
		return json.Marshal(ev.Value)
	}

	return []byte("{}"), nil
}

func (ev *EnvironmentVariableChoice) UnmarshalJSON(bytes []byte) error {
	var property Property
	err := json.Unmarshal(bytes, &property)
	if err != nil {
		var ute *json.UnmarshalTypeError
		if !errors.As(err, &ute) || ute.Value != "string" {
			return err
		}
	}

	if property != (Property{}) {
		ev.Property = &property
		return nil
	}

	var value string
	err = json.Unmarshal(bytes, &value)
	if err != nil {
		var ute *json.UnmarshalTypeError
		if !errors.As(err, &ute) || ute.Value != "object" {
			return err
		}
	}

	ev.Value = value
	return nil
}

type mlDatasetChoiceRefJSON struct {
	Ref string `json:"ref" xml:"-"`
}

func (dc MLDatasetChoice) MarshalJSON() ([]byte, error) {
	if dc.Ref != "" {
		return json.Marshal(mlDatasetChoiceRefJSON{Ref: dc.Ref})
	} else if dc.ComponentData != nil {
		return json.Marshal(dc.ComponentData)
	}

	return []byte("{}"), nil
}

func (dc *MLDatasetChoice) UnmarshalJSON(bytes []byte) error {
	var refObj mlDatasetChoiceRefJSON
	err := json.Unmarshal(bytes, &refObj)
	if err != nil {
		return err
	}

	if refObj.Ref != "" {
		dc.Ref = refObj.Ref
		return nil
	}

	var componentData ComponentData
	err = json.Unmarshal(bytes, &componentData)
	if err != nil {
		return err
	}

	if componentData != (ComponentData{}) {
		dc.ComponentData = &componentData
	}

	return nil
}

func (sv SpecVersion) MarshalJSON() ([]byte, error) {
	return json.Marshal(sv.String())
}

func (sv *SpecVersion) UnmarshalJSON(bytes []byte) error {
	var v string
	err := json.Unmarshal(bytes, &v)
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

type toolsChoiceJSON struct {
	Components *[]Component `json:"components,omitempty" xml:"-"`
	Services   *[]Service   `json:"services,omitempty" xml:"-"`
}

func (tc ToolsChoice) MarshalJSON() ([]byte, error) {
	if tc.Tools != nil && (tc.Components != nil || tc.Services != nil) {
		return nil, fmt.Errorf("either a list of tools, or an object holding components and services can be used, but not both")
	}

	if tc.Tools != nil {
		return json.Marshal(tc.Tools)
	}

	choiceJSON := toolsChoiceJSON{
		Components: tc.Components,
		Services:   tc.Services,
	}
	if choiceJSON.Components != nil || choiceJSON.Services != nil {
		return json.Marshal(choiceJSON)
	}

	return []byte(nil), nil
}

func (tc *ToolsChoice) UnmarshalJSON(bytes []byte) error {
	var choiceJSON toolsChoiceJSON
	err := json.Unmarshal(bytes, &choiceJSON)
	if err != nil {
		var typeErr *json.UnmarshalTypeError
		if !errors.As(err, &typeErr) || typeErr.Value != "array" {
			return err
		}

		var legacyTools []Tool
		err = json.Unmarshal(bytes, &legacyTools)
		if err != nil {
			return err
		}

		*tc = ToolsChoice{Tools: &legacyTools}
		return nil
	}

	if choiceJSON.Components != nil || choiceJSON.Services != nil {
		*tc = ToolsChoice{
			Components: choiceJSON.Components,
			Services:   choiceJSON.Services,
		}
	}

	return nil
}

func (eic EvidenceIdentityChoice) MarshalJSON() ([]byte, error) {
	if eic.Identity != nil && eic.Identities != nil {
		return nil, fmt.Errorf("either a single identity or an array of identities can be used, but not both")
	}
	if eic.Identity != nil {
		return json.Marshal(eic.Identity)
	} else if eic.Identities != nil {
		return json.Marshal(eic.Identities)
	}
	return []byte("null"), nil
}

func (eic *EvidenceIdentityChoice) UnmarshalJSON(data []byte) error {
	// Discriminate based on whether data is an array or a single object.
	if len(data) > 0 && data[0] == '[' {
		var identities []EvidenceIdentity
		if err := json.Unmarshal(data, &identities); err != nil {
			return err
		}
		eic.Identities = &identities
		return nil
	}
	var identity EvidenceIdentity
	if err := json.Unmarshal(data, &identity); err != nil {
		return err
	}
	eic.Identity = &identity
	return nil
}

func (l License) MarshalJSON() ([]byte, error) {
	if l.ID != "" && l.Name != "" {
		return nil, fmt.Errorf("license must have either id or name, not both")
	}
	if l.ID == "" && l.Name == "" {
		return nil, fmt.Errorf("license must have either id or name")
	}
	type Alias License
	return json.Marshal(Alias(l))
}

func (cs CertificateState) MarshalJSON() ([]byte, error) {
	if cs.Predefined != nil && cs.Custom != nil {
		return nil, fmt.Errorf("either a predefined or custom certificate state can be used, but not both")
	}
	if cs.Predefined != nil {
		return json.Marshal(cs.Predefined)
	} else if cs.Custom != nil {
		return json.Marshal(cs.Custom)
	}
	return []byte("{}"), nil
}

func (cs *CertificateState) UnmarshalJSON(data []byte) error {
	var peek struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal(data, &peek); err != nil {
		return err
	}
	if peek.State != "" {
		var p PredefinedCertificateState
		if err := json.Unmarshal(data, &p); err != nil {
			return err
		}
		cs.Predefined = &p
		return nil
	}
	var c CustomCertificateState
	if err := json.Unmarshal(data, &c); err != nil {
		return err
	}
	cs.Custom = &c
	return nil
}

func (ce CertificateExtension) MarshalJSON() ([]byte, error) {
	if ce.Common != nil && ce.Custom != nil {
		return nil, fmt.Errorf("either a common or custom certificate extension can be used, but not both")
	}
	if ce.Common != nil {
		return json.Marshal(ce.Common)
	} else if ce.Custom != nil {
		return json.Marshal(ce.Custom)
	}
	return []byte("{}"), nil
}

func (ce *CertificateExtension) UnmarshalJSON(data []byte) error {
	var peek struct {
		CommonExtensionName string `json:"commonExtensionName"`
	}
	if err := json.Unmarshal(data, &peek); err != nil {
		return err
	}
	if peek.CommonExtensionName != "" {
		var c CommonCertificateExtension
		if err := json.Unmarshal(data, &c); err != nil {
			return err
		}
		ce.Common = &c
		return nil
	}
	var c CustomCertificateExtension
	if err := json.Unmarshal(data, &c); err != nil {
		return err
	}
	ce.Custom = &c
	return nil
}

func (ac AsserterChoice) MarshalJSON() ([]byte, error) {
	if ac.Organization != nil {
		return json.Marshal(struct {
			Organization *OrganizationalEntity `json:"organization"`
		}{Organization: ac.Organization})
	} else if ac.Individual != nil {
		return json.Marshal(struct {
			Individual *OrganizationalContact `json:"individual"`
		}{Individual: ac.Individual})
	} else if ac.BOMRef != nil {
		return json.Marshal(struct {
			BOMRef *BOMReference `json:"ref"`
		}{BOMRef: ac.BOMRef})
	}
	return []byte("{}"), nil
}

func (ac *AsserterChoice) UnmarshalJSON(data []byte) error {
	var peek struct {
		Organization *json.RawMessage `json:"organization"`
		Individual   *json.RawMessage `json:"individual"`
		BOMRef       *json.RawMessage `json:"ref"`
	}
	if err := json.Unmarshal(data, &peek); err != nil {
		return err
	}
	if peek.Organization != nil {
		var org OrganizationalEntity
		if err := json.Unmarshal(*peek.Organization, &org); err != nil {
			return err
		}
		ac.Organization = &org
	} else if peek.Individual != nil {
		var contact OrganizationalContact
		if err := json.Unmarshal(*peek.Individual, &contact); err != nil {
			return err
		}
		ac.Individual = &contact
	} else if peek.BOMRef != nil {
		var ref BOMReference
		if err := json.Unmarshal(*peek.BOMRef, &ref); err != nil {
			return err
		}
		ac.BOMRef = &ref
	}
	return nil
}

func ikev2MarshalJSON(bomRef BOMReference, structured interface{}) ([]byte, error) {
	if bomRef != "" {
		return json.Marshal(string(bomRef))
	}
	return json.Marshal(structured)
}

func ikev2UnmarshalJSON(data []byte, bomRef *BOMReference, structured interface{}) error {
	if len(data) > 0 && data[0] == '"' {
		return json.Unmarshal(data, bomRef)
	}
	return json.Unmarshal(data, structured)
}

func (v IKEv2Auth) MarshalJSON() ([]byte, error) {
	type alias IKEv2Auth
	return ikev2MarshalJSON(v.BOMRef, alias(v))
}

func (v *IKEv2Auth) UnmarshalJSON(data []byte) error {
	type alias IKEv2Auth
	return ikev2UnmarshalJSON(data, &v.BOMRef, (*alias)(v))
}

func (v IKEv2Enc) MarshalJSON() ([]byte, error) {
	type alias IKEv2Enc
	return ikev2MarshalJSON(v.BOMRef, alias(v))
}

func (v *IKEv2Enc) UnmarshalJSON(data []byte) error {
	type alias IKEv2Enc
	return ikev2UnmarshalJSON(data, &v.BOMRef, (*alias)(v))
}

func (v IKEv2Integ) MarshalJSON() ([]byte, error) {
	type alias IKEv2Integ
	return ikev2MarshalJSON(v.BOMRef, alias(v))
}

func (v *IKEv2Integ) UnmarshalJSON(data []byte) error {
	type alias IKEv2Integ
	return ikev2UnmarshalJSON(data, &v.BOMRef, (*alias)(v))
}

func (v IKEv2Ke) MarshalJSON() ([]byte, error) {
	type alias IKEv2Ke
	return ikev2MarshalJSON(v.BOMRef, alias(v))
}

func (v *IKEv2Ke) UnmarshalJSON(data []byte) error {
	type alias IKEv2Ke
	return ikev2UnmarshalJSON(data, &v.BOMRef, (*alias)(v))
}

func (v IKEv2Prf) MarshalJSON() ([]byte, error) {
	type alias IKEv2Prf
	return ikev2MarshalJSON(v.BOMRef, alias(v))
}

func (v *IKEv2Prf) UnmarshalJSON(data []byte) error {
	type alias IKEv2Prf
	return ikev2UnmarshalJSON(data, &v.BOMRef, (*alias)(v))
}

func (pc PatentChoice) MarshalJSON() ([]byte, error) {
	if pc.Patent != nil {
		return json.Marshal(pc.Patent)
	} else if pc.PatentFamily != nil {
		return json.Marshal(pc.PatentFamily)
	}

	return []byte("{}"), nil
}

func (pc *PatentChoice) UnmarshalJSON(data []byte) error {
	// Distinguish patent from patentFamily by checking for the familyId field,
	// which is the required field unique to patentFamily.
	var peek struct {
		FamilyID string `json:"familyId"`
	}
	if err := json.Unmarshal(data, &peek); err != nil {
		return err
	}

	if peek.FamilyID != "" {
		var pf PatentFamily
		if err := json.Unmarshal(data, &pf); err != nil {
			return err
		}
		pc.PatentFamily = &pf
		return nil
	}

	var p Patent
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	pc.Patent = &p
	return nil
}

var jsonSchemas = map[SpecVersion]string{
	SpecVersion1_0: "",
	SpecVersion1_1: "",
	SpecVersion1_2: "http://cyclonedx.org/schema/bom-1.2.schema.json",
	SpecVersion1_3: "http://cyclonedx.org/schema/bom-1.3.schema.json",
	SpecVersion1_4: "http://cyclonedx.org/schema/bom-1.4.schema.json",
	SpecVersion1_5: "http://cyclonedx.org/schema/bom-1.5.schema.json",
	SpecVersion1_6: "http://cyclonedx.org/schema/bom-1.6.schema.json",
	SpecVersion1_7: "http://cyclonedx.org/schema/bom-1.7.schema.json",
}
