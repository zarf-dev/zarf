// SPDX-License-Identifier: BSD-3-Clause
// SPDX-FileCopyrightText: 2017 FIRST.ORG, INC.
//
// THIS FILE IS MACHINE GENERATED. EDIT WITH CARE!

package csaf

// CVSS3AttackComplexity represents the attackComplexityType in CVSS3.
type CVSS3AttackComplexity string

const (
	// CVSS3AttackComplexityHigh is a constant for "HIGH".
	CVSS3AttackComplexityHigh CVSS3AttackComplexity = "HIGH"
	// CVSS3AttackComplexityLow is a constant for "LOW".
	CVSS3AttackComplexityLow CVSS3AttackComplexity = "LOW"
)

var cvss3AttackComplexityPattern = alternativesUnmarshal(
	string(CVSS3AttackComplexityHigh),
	string(CVSS3AttackComplexityLow),
)

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
func (e *CVSS3AttackComplexity) UnmarshalText(data []byte) error {
	s, err := cvss3AttackComplexityPattern(data)
	if err == nil {
		*e = CVSS3AttackComplexity(s)
	}
	return err
}

// CVSS3AttackVector represents the attackVectorType in CVSS3.
type CVSS3AttackVector string

const (
	// CVSS3AttackVectorNetwork is a constant for "NETWORK".
	CVSS3AttackVectorNetwork CVSS3AttackVector = "NETWORK"
	// CVSS3AttackVectorAdjacentNetwork is a constant for "ADJACENT_NETWORK".
	CVSS3AttackVectorAdjacentNetwork CVSS3AttackVector = "ADJACENT_NETWORK"
	// CVSS3AttackVectorLocal is a constant for "LOCAL".
	CVSS3AttackVectorLocal CVSS3AttackVector = "LOCAL"
	// CVSS3AttackVectorPhysical is a constant for "PHYSICAL".
	CVSS3AttackVectorPhysical CVSS3AttackVector = "PHYSICAL"
)

var cvss3AttackVectorPattern = alternativesUnmarshal(
	string(CVSS3AttackVectorNetwork),
	string(CVSS3AttackVectorAdjacentNetwork),
	string(CVSS3AttackVectorLocal),
	string(CVSS3AttackVectorPhysical),
)

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
func (e *CVSS3AttackVector) UnmarshalText(data []byte) error {
	s, err := cvss3AttackVectorPattern(data)
	if err == nil {
		*e = CVSS3AttackVector(s)
	}
	return err
}

// CVSS3CiaRequirement represents the ciaRequirementType in CVSS3.
type CVSS3CiaRequirement string

const (
	// CVSS3CiaRequirementLow is a constant for "LOW".
	CVSS3CiaRequirementLow CVSS3CiaRequirement = "LOW"
	// CVSS3CiaRequirementMedium is a constant for "MEDIUM".
	CVSS3CiaRequirementMedium CVSS3CiaRequirement = "MEDIUM"
	// CVSS3CiaRequirementHigh is a constant for "HIGH".
	CVSS3CiaRequirementHigh CVSS3CiaRequirement = "HIGH"
	// CVSS3CiaRequirementNotDefined is a constant for "NOT_DEFINED".
	CVSS3CiaRequirementNotDefined CVSS3CiaRequirement = "NOT_DEFINED"
)

var cvss3CiaRequirementPattern = alternativesUnmarshal(
	string(CVSS3CiaRequirementLow),
	string(CVSS3CiaRequirementMedium),
	string(CVSS3CiaRequirementHigh),
	string(CVSS3CiaRequirementNotDefined),
)

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
func (e *CVSS3CiaRequirement) UnmarshalText(data []byte) error {
	s, err := cvss3CiaRequirementPattern(data)
	if err == nil {
		*e = CVSS3CiaRequirement(s)
	}
	return err
}

// CVSS3Cia represents the ciaType in CVSS3.
type CVSS3Cia string

const (
	// CVSS3CiaNone is a constant for "NONE".
	CVSS3CiaNone CVSS3Cia = "NONE"
	// CVSS3CiaLow is a constant for "LOW".
	CVSS3CiaLow CVSS3Cia = "LOW"
	// CVSS3CiaHigh is a constant for "HIGH".
	CVSS3CiaHigh CVSS3Cia = "HIGH"
)

var cvss3CiaPattern = alternativesUnmarshal(
	string(CVSS3CiaNone),
	string(CVSS3CiaLow),
	string(CVSS3CiaHigh),
)

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
func (e *CVSS3Cia) UnmarshalText(data []byte) error {
	s, err := cvss3CiaPattern(data)
	if err == nil {
		*e = CVSS3Cia(s)
	}
	return err
}

// CVSS3Confidence represents the confidenceType in CVSS3.
type CVSS3Confidence string

const (
	// CVSS3ConfidenceUnknown is a constant for "UNKNOWN".
	CVSS3ConfidenceUnknown CVSS3Confidence = "UNKNOWN"
	// CVSS3ConfidenceReasonable is a constant for "REASONABLE".
	CVSS3ConfidenceReasonable CVSS3Confidence = "REASONABLE"
	// CVSS3ConfidenceConfirmed is a constant for "CONFIRMED".
	CVSS3ConfidenceConfirmed CVSS3Confidence = "CONFIRMED"
	// CVSS3ConfidenceNotDefined is a constant for "NOT_DEFINED".
	CVSS3ConfidenceNotDefined CVSS3Confidence = "NOT_DEFINED"
)

var cvss3ConfidencePattern = alternativesUnmarshal(
	string(CVSS3ConfidenceUnknown),
	string(CVSS3ConfidenceReasonable),
	string(CVSS3ConfidenceConfirmed),
	string(CVSS3ConfidenceNotDefined),
)

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
func (e *CVSS3Confidence) UnmarshalText(data []byte) error {
	s, err := cvss3ConfidencePattern(data)
	if err == nil {
		*e = CVSS3Confidence(s)
	}
	return err
}

// CVSS3ExploitCodeMaturity represents the exploitCodeMaturityType in CVSS3.
type CVSS3ExploitCodeMaturity string

const (
	// CVSS3ExploitCodeMaturityUnproven is a constant for "UNPROVEN".
	CVSS3ExploitCodeMaturityUnproven CVSS3ExploitCodeMaturity = "UNPROVEN"
	// CVSS3ExploitCodeMaturityProofOfConcept is a constant for "PROOF_OF_CONCEPT".
	CVSS3ExploitCodeMaturityProofOfConcept CVSS3ExploitCodeMaturity = "PROOF_OF_CONCEPT"
	// CVSS3ExploitCodeMaturityFunctional is a constant for "FUNCTIONAL".
	CVSS3ExploitCodeMaturityFunctional CVSS3ExploitCodeMaturity = "FUNCTIONAL"
	// CVSS3ExploitCodeMaturityHigh is a constant for "HIGH".
	CVSS3ExploitCodeMaturityHigh CVSS3ExploitCodeMaturity = "HIGH"
	// CVSS3ExploitCodeMaturityNotDefined is a constant for "NOT_DEFINED".
	CVSS3ExploitCodeMaturityNotDefined CVSS3ExploitCodeMaturity = "NOT_DEFINED"
)

var cvss3ExploitCodeMaturityPattern = alternativesUnmarshal(
	string(CVSS3ExploitCodeMaturityUnproven),
	string(CVSS3ExploitCodeMaturityProofOfConcept),
	string(CVSS3ExploitCodeMaturityFunctional),
	string(CVSS3ExploitCodeMaturityHigh),
	string(CVSS3ExploitCodeMaturityNotDefined),
)

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
func (e *CVSS3ExploitCodeMaturity) UnmarshalText(data []byte) error {
	s, err := cvss3ExploitCodeMaturityPattern(data)
	if err == nil {
		*e = CVSS3ExploitCodeMaturity(s)
	}
	return err
}

// CVSS3ModifiedAttackComplexity represents the modifiedAttackComplexityType in CVSS3.
type CVSS3ModifiedAttackComplexity string

const (
	// CVSS3ModifiedAttackComplexityHigh is a constant for "HIGH".
	CVSS3ModifiedAttackComplexityHigh CVSS3ModifiedAttackComplexity = "HIGH"
	// CVSS3ModifiedAttackComplexityLow is a constant for "LOW".
	CVSS3ModifiedAttackComplexityLow CVSS3ModifiedAttackComplexity = "LOW"
	// CVSS3ModifiedAttackComplexityNotDefined is a constant for "NOT_DEFINED".
	CVSS3ModifiedAttackComplexityNotDefined CVSS3ModifiedAttackComplexity = "NOT_DEFINED"
)

var cvss3ModifiedAttackComplexityPattern = alternativesUnmarshal(
	string(CVSS3ModifiedAttackComplexityHigh),
	string(CVSS3ModifiedAttackComplexityLow),
	string(CVSS3ModifiedAttackComplexityNotDefined),
)

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
func (e *CVSS3ModifiedAttackComplexity) UnmarshalText(data []byte) error {
	s, err := cvss3ModifiedAttackComplexityPattern(data)
	if err == nil {
		*e = CVSS3ModifiedAttackComplexity(s)
	}
	return err
}

// CVSS3ModifiedAttackVector represents the modifiedAttackVectorType in CVSS3.
type CVSS3ModifiedAttackVector string

const (
	// CVSS3ModifiedAttackVectorNetwork is a constant for "NETWORK".
	CVSS3ModifiedAttackVectorNetwork CVSS3ModifiedAttackVector = "NETWORK"
	// CVSS3ModifiedAttackVectorAdjacentNetwork is a constant for "ADJACENT_NETWORK".
	CVSS3ModifiedAttackVectorAdjacentNetwork CVSS3ModifiedAttackVector = "ADJACENT_NETWORK"
	// CVSS3ModifiedAttackVectorLocal is a constant for "LOCAL".
	CVSS3ModifiedAttackVectorLocal CVSS3ModifiedAttackVector = "LOCAL"
	// CVSS3ModifiedAttackVectorPhysical is a constant for "PHYSICAL".
	CVSS3ModifiedAttackVectorPhysical CVSS3ModifiedAttackVector = "PHYSICAL"
	// CVSS3ModifiedAttackVectorNotDefined is a constant for "NOT_DEFINED".
	CVSS3ModifiedAttackVectorNotDefined CVSS3ModifiedAttackVector = "NOT_DEFINED"
)

var cvss3ModifiedAttackVectorPattern = alternativesUnmarshal(
	string(CVSS3ModifiedAttackVectorNetwork),
	string(CVSS3ModifiedAttackVectorAdjacentNetwork),
	string(CVSS3ModifiedAttackVectorLocal),
	string(CVSS3ModifiedAttackVectorPhysical),
	string(CVSS3ModifiedAttackVectorNotDefined),
)

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
func (e *CVSS3ModifiedAttackVector) UnmarshalText(data []byte) error {
	s, err := cvss3ModifiedAttackVectorPattern(data)
	if err == nil {
		*e = CVSS3ModifiedAttackVector(s)
	}
	return err
}

// CVSS3ModifiedCia represents the modifiedCiaType in CVSS3.
type CVSS3ModifiedCia string

const (
	// CVSS3ModifiedCiaNone is a constant for "NONE".
	CVSS3ModifiedCiaNone CVSS3ModifiedCia = "NONE"
	// CVSS3ModifiedCiaLow is a constant for "LOW".
	CVSS3ModifiedCiaLow CVSS3ModifiedCia = "LOW"
	// CVSS3ModifiedCiaHigh is a constant for "HIGH".
	CVSS3ModifiedCiaHigh CVSS3ModifiedCia = "HIGH"
	// CVSS3ModifiedCiaNotDefined is a constant for "NOT_DEFINED".
	CVSS3ModifiedCiaNotDefined CVSS3ModifiedCia = "NOT_DEFINED"
)

var cvss3ModifiedCiaPattern = alternativesUnmarshal(
	string(CVSS3ModifiedCiaNone),
	string(CVSS3ModifiedCiaLow),
	string(CVSS3ModifiedCiaHigh),
	string(CVSS3ModifiedCiaNotDefined),
)

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
func (e *CVSS3ModifiedCia) UnmarshalText(data []byte) error {
	s, err := cvss3ModifiedCiaPattern(data)
	if err == nil {
		*e = CVSS3ModifiedCia(s)
	}
	return err
}

// CVSS3ModifiedPrivilegesRequired represents the modifiedPrivilegesRequiredType in CVSS3.
type CVSS3ModifiedPrivilegesRequired string

const (
	// CVSS3ModifiedPrivilegesRequiredHigh is a constant for "HIGH".
	CVSS3ModifiedPrivilegesRequiredHigh CVSS3ModifiedPrivilegesRequired = "HIGH"
	// CVSS3ModifiedPrivilegesRequiredLow is a constant for "LOW".
	CVSS3ModifiedPrivilegesRequiredLow CVSS3ModifiedPrivilegesRequired = "LOW"
	// CVSS3ModifiedPrivilegesRequiredNone is a constant for "NONE".
	CVSS3ModifiedPrivilegesRequiredNone CVSS3ModifiedPrivilegesRequired = "NONE"
	// CVSS3ModifiedPrivilegesRequiredNotDefined is a constant for "NOT_DEFINED".
	CVSS3ModifiedPrivilegesRequiredNotDefined CVSS3ModifiedPrivilegesRequired = "NOT_DEFINED"
)

var cvss3ModifiedPrivilegesRequiredPattern = alternativesUnmarshal(
	string(CVSS3ModifiedPrivilegesRequiredHigh),
	string(CVSS3ModifiedPrivilegesRequiredLow),
	string(CVSS3ModifiedPrivilegesRequiredNone),
	string(CVSS3ModifiedPrivilegesRequiredNotDefined),
)

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
func (e *CVSS3ModifiedPrivilegesRequired) UnmarshalText(data []byte) error {
	s, err := cvss3ModifiedPrivilegesRequiredPattern(data)
	if err == nil {
		*e = CVSS3ModifiedPrivilegesRequired(s)
	}
	return err
}

// CVSS3ModifiedScope represents the modifiedScopeType in CVSS3.
type CVSS3ModifiedScope string

const (
	// CVSS3ModifiedScopeUnchanged is a constant for "UNCHANGED".
	CVSS3ModifiedScopeUnchanged CVSS3ModifiedScope = "UNCHANGED"
	// CVSS3ModifiedScopeChanged is a constant for "CHANGED".
	CVSS3ModifiedScopeChanged CVSS3ModifiedScope = "CHANGED"
	// CVSS3ModifiedScopeNotDefined is a constant for "NOT_DEFINED".
	CVSS3ModifiedScopeNotDefined CVSS3ModifiedScope = "NOT_DEFINED"
)

var cvss3ModifiedScopePattern = alternativesUnmarshal(
	string(CVSS3ModifiedScopeUnchanged),
	string(CVSS3ModifiedScopeChanged),
	string(CVSS3ModifiedScopeNotDefined),
)

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
func (e *CVSS3ModifiedScope) UnmarshalText(data []byte) error {
	s, err := cvss3ModifiedScopePattern(data)
	if err == nil {
		*e = CVSS3ModifiedScope(s)
	}
	return err
}

// CVSS3ModifiedUserInteraction represents the modifiedUserInteractionType in CVSS3.
type CVSS3ModifiedUserInteraction string

const (
	// CVSS3ModifiedUserInteractionNone is a constant for "NONE".
	CVSS3ModifiedUserInteractionNone CVSS3ModifiedUserInteraction = "NONE"
	// CVSS3ModifiedUserInteractionRequired is a constant for "REQUIRED".
	CVSS3ModifiedUserInteractionRequired CVSS3ModifiedUserInteraction = "REQUIRED"
	// CVSS3ModifiedUserInteractionNotDefined is a constant for "NOT_DEFINED".
	CVSS3ModifiedUserInteractionNotDefined CVSS3ModifiedUserInteraction = "NOT_DEFINED"
)

var cvss3ModifiedUserInteractionPattern = alternativesUnmarshal(
	string(CVSS3ModifiedUserInteractionNone),
	string(CVSS3ModifiedUserInteractionRequired),
	string(CVSS3ModifiedUserInteractionNotDefined),
)

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
func (e *CVSS3ModifiedUserInteraction) UnmarshalText(data []byte) error {
	s, err := cvss3ModifiedUserInteractionPattern(data)
	if err == nil {
		*e = CVSS3ModifiedUserInteraction(s)
	}
	return err
}

// CVSS3PrivilegesRequired represents the privilegesRequiredType in CVSS3.
type CVSS3PrivilegesRequired string

const (
	// CVSS3PrivilegesRequiredHigh is a constant for "HIGH".
	CVSS3PrivilegesRequiredHigh CVSS3PrivilegesRequired = "HIGH"
	// CVSS3PrivilegesRequiredLow is a constant for "LOW".
	CVSS3PrivilegesRequiredLow CVSS3PrivilegesRequired = "LOW"
	// CVSS3PrivilegesRequiredNone is a constant for "NONE".
	CVSS3PrivilegesRequiredNone CVSS3PrivilegesRequired = "NONE"
)

var cvss3PrivilegesRequiredPattern = alternativesUnmarshal(
	string(CVSS3PrivilegesRequiredHigh),
	string(CVSS3PrivilegesRequiredLow),
	string(CVSS3PrivilegesRequiredNone),
)

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
func (e *CVSS3PrivilegesRequired) UnmarshalText(data []byte) error {
	s, err := cvss3PrivilegesRequiredPattern(data)
	if err == nil {
		*e = CVSS3PrivilegesRequired(s)
	}
	return err
}

// CVSS3RemediationLevel represents the remediationLevelType in CVSS3.
type CVSS3RemediationLevel string

const (
	// CVSS3RemediationLevelOfficialFix is a constant for "OFFICIAL_FIX".
	CVSS3RemediationLevelOfficialFix CVSS3RemediationLevel = "OFFICIAL_FIX"
	// CVSS3RemediationLevelTemporaryFix is a constant for "TEMPORARY_FIX".
	CVSS3RemediationLevelTemporaryFix CVSS3RemediationLevel = "TEMPORARY_FIX"
	// CVSS3RemediationLevelWorkaround is a constant for "WORKAROUND".
	CVSS3RemediationLevelWorkaround CVSS3RemediationLevel = "WORKAROUND"
	// CVSS3RemediationLevelUnavailable is a constant for "UNAVAILABLE".
	CVSS3RemediationLevelUnavailable CVSS3RemediationLevel = "UNAVAILABLE"
	// CVSS3RemediationLevelNotDefined is a constant for "NOT_DEFINED".
	CVSS3RemediationLevelNotDefined CVSS3RemediationLevel = "NOT_DEFINED"
)

var cvss3RemediationLevelPattern = alternativesUnmarshal(
	string(CVSS3RemediationLevelOfficialFix),
	string(CVSS3RemediationLevelTemporaryFix),
	string(CVSS3RemediationLevelWorkaround),
	string(CVSS3RemediationLevelUnavailable),
	string(CVSS3RemediationLevelNotDefined),
)

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
func (e *CVSS3RemediationLevel) UnmarshalText(data []byte) error {
	s, err := cvss3RemediationLevelPattern(data)
	if err == nil {
		*e = CVSS3RemediationLevel(s)
	}
	return err
}

// CVSS3Scope represents the scopeType in CVSS3.
type CVSS3Scope string

const (
	// CVSS3ScopeUnchanged is a constant for "UNCHANGED".
	CVSS3ScopeUnchanged CVSS3Scope = "UNCHANGED"
	// CVSS3ScopeChanged is a constant for "CHANGED".
	CVSS3ScopeChanged CVSS3Scope = "CHANGED"
)

var cvss3ScopePattern = alternativesUnmarshal(
	string(CVSS3ScopeUnchanged),
	string(CVSS3ScopeChanged),
)

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
func (e *CVSS3Scope) UnmarshalText(data []byte) error {
	s, err := cvss3ScopePattern(data)
	if err == nil {
		*e = CVSS3Scope(s)
	}
	return err
}

// CVSS3Severity represents the severityType in CVSS3.
type CVSS3Severity string

const (
	// CVSS3SeverityNone is a constant for "NONE".
	CVSS3SeverityNone CVSS3Severity = "NONE"
	// CVSS3SeverityLow is a constant for "LOW".
	CVSS3SeverityLow CVSS3Severity = "LOW"
	// CVSS3SeverityMedium is a constant for "MEDIUM".
	CVSS3SeverityMedium CVSS3Severity = "MEDIUM"
	// CVSS3SeverityHigh is a constant for "HIGH".
	CVSS3SeverityHigh CVSS3Severity = "HIGH"
	// CVSS3SeverityCritical is a constant for "CRITICAL".
	CVSS3SeverityCritical CVSS3Severity = "CRITICAL"
)

var cvss3SeverityPattern = alternativesUnmarshal(
	string(CVSS3SeverityNone),
	string(CVSS3SeverityLow),
	string(CVSS3SeverityMedium),
	string(CVSS3SeverityHigh),
	string(CVSS3SeverityCritical),
)

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
func (e *CVSS3Severity) UnmarshalText(data []byte) error {
	s, err := cvss3SeverityPattern(data)
	if err == nil {
		*e = CVSS3Severity(s)
	}
	return err
}

// CVSS3UserInteraction represents the userInteractionType in CVSS3.
type CVSS3UserInteraction string

const (
	// CVSS3UserInteractionNone is a constant for "NONE".
	CVSS3UserInteractionNone CVSS3UserInteraction = "NONE"
	// CVSS3UserInteractionRequired is a constant for "REQUIRED".
	CVSS3UserInteractionRequired CVSS3UserInteraction = "REQUIRED"
)

var cvss3UserInteractionPattern = alternativesUnmarshal(
	string(CVSS3UserInteractionNone),
	string(CVSS3UserInteractionRequired),
)

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
func (e *CVSS3UserInteraction) UnmarshalText(data []byte) error {
	s, err := cvss3UserInteractionPattern(data)
	if err == nil {
		*e = CVSS3UserInteraction(s)
	}
	return err
}
