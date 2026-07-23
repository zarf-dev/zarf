/*
Copyright 2023 The OpenVEX Authors
SPDX-License-Identifier: Apache-2.0
*/

package vex

// Justification describes why a given component is not affected by a
// vulnerability.
type Justification string

const (
	// ComponentNotPresent means the vulnerable component is not included in the artifact.
	//
	// ComponentNotPresent is a strong justification that the artifact is not affected.
	ComponentNotPresent Justification = "component_not_present"

	// VulnerableCodeNotPresent means the vulnerable component is included in
	// artifact, but the vulnerable code is not present. Typically, this case occurs
	// when source code is configured or built in a way that excluded the vulnerable
	// code.
	//
	// VulnerableCodeNotPresent is a strong justification that the artifact is not affected.
	VulnerableCodeNotPresent Justification = "vulnerable_code_not_present"

	// VulnerableCodeNotInExecutePath means the vulnerable code (likely in
	// [subcomponent_id]) can not be executed as it is used by [product_id].
	// Typically, this case occurs when [product_id] includes the vulnerable
	// [subcomponent_id] and the vulnerable code but does not call or use the
	// vulnerable code.
	VulnerableCodeNotInExecutePath Justification = "vulnerable_code_not_in_execute_path"

	// VulnerableCodeCannotBeControlledByAdversary means the vulnerable code cannot
	// be controlled by an attacker to exploit the vulnerability.
	//
	// This justification could be difficult to prove conclusively.
	VulnerableCodeCannotBeControlledByAdversary Justification = "vulnerable_code_cannot_be_controlled_by_adversary"

	// InlineMitigationsAlreadyExist means [product_id] includes built-in protections
	// or features that prevent exploitation of the vulnerability. These built-in
	// protections cannot be subverted by the attacker and cannot be configured or
	// disabled by the user. These mitigations completely prevent exploitation based
	// on known attack vectors.
	//
	// This justification could be difficult to prove conclusively. History is
	// littered with examples of mitigation bypasses, typically involving minor
	// modifications of existing exploit code.
	InlineMitigationsAlreadyExist Justification = "inline_mitigations_already_exist"
)

// Justifications returns a list of the valid Justification values.
func Justifications() []string {
	return []string{
		string(ComponentNotPresent),
		string(VulnerableCodeNotPresent),
		string(VulnerableCodeNotInExecutePath),
		string(VulnerableCodeCannotBeControlledByAdversary),
		string(InlineMitigationsAlreadyExist),
	}
}

// Valid returns a bool indicating whether the Justification value is equal to
// one of the enumerated allowed values for Justification.
func (j Justification) Valid() bool {
	switch j {
	case ComponentNotPresent,
		VulnerableCodeNotPresent,
		VulnerableCodeNotInExecutePath,
		VulnerableCodeCannotBeControlledByAdversary,
		InlineMitigationsAlreadyExist:

		return true

	default:

		return false
	}
}
