// SPDX-License-Identifier: BSD-3-Clause
// SPDX-FileCopyrightText: 2017 FIRST.ORG, INC.
//
// THIS FILE IS MACHINE GENERATED. EDIT WITH CARE!

package csaf

// CVSS20AccessComplexity represents the accessComplexityType in CVSS20.
type CVSS20AccessComplexity string

const (
	// CVSS20AccessComplexityHigh is a constant for "HIGH".
	CVSS20AccessComplexityHigh CVSS20AccessComplexity = "HIGH"
	// CVSS20AccessComplexityMedium is a constant for "MEDIUM".
	CVSS20AccessComplexityMedium CVSS20AccessComplexity = "MEDIUM"
	// CVSS20AccessComplexityLow is a constant for "LOW".
	CVSS20AccessComplexityLow CVSS20AccessComplexity = "LOW"
)

var cvss20AccessComplexityPattern = alternativesUnmarshal(
	string(CVSS20AccessComplexityHigh),
	string(CVSS20AccessComplexityMedium),
	string(CVSS20AccessComplexityLow),
)

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
func (e *CVSS20AccessComplexity) UnmarshalText(data []byte) error {
	s, err := cvss20AccessComplexityPattern(data)
	if err == nil {
		*e = CVSS20AccessComplexity(s)
	}
	return err
}

// CVSS20AccessVector represents the accessVectorType in CVSS20.
type CVSS20AccessVector string

const (
	// CVSS20AccessVectorNetwork is a constant for "NETWORK".
	CVSS20AccessVectorNetwork CVSS20AccessVector = "NETWORK"
	// CVSS20AccessVectorAdjacentNetwork is a constant for "ADJACENT_NETWORK".
	CVSS20AccessVectorAdjacentNetwork CVSS20AccessVector = "ADJACENT_NETWORK"
	// CVSS20AccessVectorLocal is a constant for "LOCAL".
	CVSS20AccessVectorLocal CVSS20AccessVector = "LOCAL"
)

var cvss20AccessVectorPattern = alternativesUnmarshal(
	string(CVSS20AccessVectorNetwork),
	string(CVSS20AccessVectorAdjacentNetwork),
	string(CVSS20AccessVectorLocal),
)

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
func (e *CVSS20AccessVector) UnmarshalText(data []byte) error {
	s, err := cvss20AccessVectorPattern(data)
	if err == nil {
		*e = CVSS20AccessVector(s)
	}
	return err
}

// CVSS20Authentication represents the authenticationType in CVSS20.
type CVSS20Authentication string

const (
	// CVSS20AuthenticationMultiple is a constant for "MULTIPLE".
	CVSS20AuthenticationMultiple CVSS20Authentication = "MULTIPLE"
	// CVSS20AuthenticationSingle is a constant for "SINGLE".
	CVSS20AuthenticationSingle CVSS20Authentication = "SINGLE"
	// CVSS20AuthenticationNone is a constant for "NONE".
	CVSS20AuthenticationNone CVSS20Authentication = "NONE"
)

var cvss20AuthenticationPattern = alternativesUnmarshal(
	string(CVSS20AuthenticationMultiple),
	string(CVSS20AuthenticationSingle),
	string(CVSS20AuthenticationNone),
)

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
func (e *CVSS20Authentication) UnmarshalText(data []byte) error {
	s, err := cvss20AuthenticationPattern(data)
	if err == nil {
		*e = CVSS20Authentication(s)
	}
	return err
}

// CVSS20CiaRequirement represents the ciaRequirementType in CVSS20.
type CVSS20CiaRequirement string

const (
	// CVSS20CiaRequirementLow is a constant for "LOW".
	CVSS20CiaRequirementLow CVSS20CiaRequirement = "LOW"
	// CVSS20CiaRequirementMedium is a constant for "MEDIUM".
	CVSS20CiaRequirementMedium CVSS20CiaRequirement = "MEDIUM"
	// CVSS20CiaRequirementHigh is a constant for "HIGH".
	CVSS20CiaRequirementHigh CVSS20CiaRequirement = "HIGH"
	// CVSS20CiaRequirementNotDefined is a constant for "NOT_DEFINED".
	CVSS20CiaRequirementNotDefined CVSS20CiaRequirement = "NOT_DEFINED"
)

var cvss20CiaRequirementPattern = alternativesUnmarshal(
	string(CVSS20CiaRequirementLow),
	string(CVSS20CiaRequirementMedium),
	string(CVSS20CiaRequirementHigh),
	string(CVSS20CiaRequirementNotDefined),
)

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
func (e *CVSS20CiaRequirement) UnmarshalText(data []byte) error {
	s, err := cvss20CiaRequirementPattern(data)
	if err == nil {
		*e = CVSS20CiaRequirement(s)
	}
	return err
}

// CVSS20Cia represents the ciaType in CVSS20.
type CVSS20Cia string

const (
	// CVSS20CiaNone is a constant for "NONE".
	CVSS20CiaNone CVSS20Cia = "NONE"
	// CVSS20CiaPartial is a constant for "PARTIAL".
	CVSS20CiaPartial CVSS20Cia = "PARTIAL"
	// CVSS20CiaComplete is a constant for "COMPLETE".
	CVSS20CiaComplete CVSS20Cia = "COMPLETE"
)

var cvss20CiaPattern = alternativesUnmarshal(
	string(CVSS20CiaNone),
	string(CVSS20CiaPartial),
	string(CVSS20CiaComplete),
)

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
func (e *CVSS20Cia) UnmarshalText(data []byte) error {
	s, err := cvss20CiaPattern(data)
	if err == nil {
		*e = CVSS20Cia(s)
	}
	return err
}

// CVSS20CollateralDamagePotential represents the collateralDamagePotentialType in CVSS20.
type CVSS20CollateralDamagePotential string

const (
	// CVSS20CollateralDamagePotentialNone is a constant for "NONE".
	CVSS20CollateralDamagePotentialNone CVSS20CollateralDamagePotential = "NONE"
	// CVSS20CollateralDamagePotentialLow is a constant for "LOW".
	CVSS20CollateralDamagePotentialLow CVSS20CollateralDamagePotential = "LOW"
	// CVSS20CollateralDamagePotentialLowMedium is a constant for "LOW_MEDIUM".
	CVSS20CollateralDamagePotentialLowMedium CVSS20CollateralDamagePotential = "LOW_MEDIUM"
	// CVSS20CollateralDamagePotentialMediumHigh is a constant for "MEDIUM_HIGH".
	CVSS20CollateralDamagePotentialMediumHigh CVSS20CollateralDamagePotential = "MEDIUM_HIGH"
	// CVSS20CollateralDamagePotentialHigh is a constant for "HIGH".
	CVSS20CollateralDamagePotentialHigh CVSS20CollateralDamagePotential = "HIGH"
	// CVSS20CollateralDamagePotentialNotDefined is a constant for "NOT_DEFINED".
	CVSS20CollateralDamagePotentialNotDefined CVSS20CollateralDamagePotential = "NOT_DEFINED"
)

var cvss20CollateralDamagePotentialPattern = alternativesUnmarshal(
	string(CVSS20CollateralDamagePotentialNone),
	string(CVSS20CollateralDamagePotentialLow),
	string(CVSS20CollateralDamagePotentialLowMedium),
	string(CVSS20CollateralDamagePotentialMediumHigh),
	string(CVSS20CollateralDamagePotentialHigh),
	string(CVSS20CollateralDamagePotentialNotDefined),
)

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
func (e *CVSS20CollateralDamagePotential) UnmarshalText(data []byte) error {
	s, err := cvss20CollateralDamagePotentialPattern(data)
	if err == nil {
		*e = CVSS20CollateralDamagePotential(s)
	}
	return err
}

// CVSS20Exploitability represents the exploitabilityType in CVSS20.
type CVSS20Exploitability string

const (
	// CVSS20ExploitabilityUnproven is a constant for "UNPROVEN".
	CVSS20ExploitabilityUnproven CVSS20Exploitability = "UNPROVEN"
	// CVSS20ExploitabilityProofOfConcept is a constant for "PROOF_OF_CONCEPT".
	CVSS20ExploitabilityProofOfConcept CVSS20Exploitability = "PROOF_OF_CONCEPT"
	// CVSS20ExploitabilityFunctional is a constant for "FUNCTIONAL".
	CVSS20ExploitabilityFunctional CVSS20Exploitability = "FUNCTIONAL"
	// CVSS20ExploitabilityHigh is a constant for "HIGH".
	CVSS20ExploitabilityHigh CVSS20Exploitability = "HIGH"
	// CVSS20ExploitabilityNotDefined is a constant for "NOT_DEFINED".
	CVSS20ExploitabilityNotDefined CVSS20Exploitability = "NOT_DEFINED"
)

var cvss20ExploitabilityPattern = alternativesUnmarshal(
	string(CVSS20ExploitabilityUnproven),
	string(CVSS20ExploitabilityProofOfConcept),
	string(CVSS20ExploitabilityFunctional),
	string(CVSS20ExploitabilityHigh),
	string(CVSS20ExploitabilityNotDefined),
)

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
func (e *CVSS20Exploitability) UnmarshalText(data []byte) error {
	s, err := cvss20ExploitabilityPattern(data)
	if err == nil {
		*e = CVSS20Exploitability(s)
	}
	return err
}

// CVSS20RemediationLevel represents the remediationLevelType in CVSS20.
type CVSS20RemediationLevel string

const (
	// CVSS20RemediationLevelOfficialFix is a constant for "OFFICIAL_FIX".
	CVSS20RemediationLevelOfficialFix CVSS20RemediationLevel = "OFFICIAL_FIX"
	// CVSS20RemediationLevelTemporaryFix is a constant for "TEMPORARY_FIX".
	CVSS20RemediationLevelTemporaryFix CVSS20RemediationLevel = "TEMPORARY_FIX"
	// CVSS20RemediationLevelWorkaround is a constant for "WORKAROUND".
	CVSS20RemediationLevelWorkaround CVSS20RemediationLevel = "WORKAROUND"
	// CVSS20RemediationLevelUnavailable is a constant for "UNAVAILABLE".
	CVSS20RemediationLevelUnavailable CVSS20RemediationLevel = "UNAVAILABLE"
	// CVSS20RemediationLevelNotDefined is a constant for "NOT_DEFINED".
	CVSS20RemediationLevelNotDefined CVSS20RemediationLevel = "NOT_DEFINED"
)

var cvss20RemediationLevelPattern = alternativesUnmarshal(
	string(CVSS20RemediationLevelOfficialFix),
	string(CVSS20RemediationLevelTemporaryFix),
	string(CVSS20RemediationLevelWorkaround),
	string(CVSS20RemediationLevelUnavailable),
	string(CVSS20RemediationLevelNotDefined),
)

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
func (e *CVSS20RemediationLevel) UnmarshalText(data []byte) error {
	s, err := cvss20RemediationLevelPattern(data)
	if err == nil {
		*e = CVSS20RemediationLevel(s)
	}
	return err
}

// CVSS20ReportConfidence represents the reportConfidenceType in CVSS20.
type CVSS20ReportConfidence string

const (
	// CVSS20ReportConfidenceUnconfirmed is a constant for "UNCONFIRMED".
	CVSS20ReportConfidenceUnconfirmed CVSS20ReportConfidence = "UNCONFIRMED"
	// CVSS20ReportConfidenceUncorroborated is a constant for "UNCORROBORATED".
	CVSS20ReportConfidenceUncorroborated CVSS20ReportConfidence = "UNCORROBORATED"
	// CVSS20ReportConfidenceConfirmed is a constant for "CONFIRMED".
	CVSS20ReportConfidenceConfirmed CVSS20ReportConfidence = "CONFIRMED"
	// CVSS20ReportConfidenceNotDefined is a constant for "NOT_DEFINED".
	CVSS20ReportConfidenceNotDefined CVSS20ReportConfidence = "NOT_DEFINED"
)

var cvss20ReportConfidencePattern = alternativesUnmarshal(
	string(CVSS20ReportConfidenceUnconfirmed),
	string(CVSS20ReportConfidenceUncorroborated),
	string(CVSS20ReportConfidenceConfirmed),
	string(CVSS20ReportConfidenceNotDefined),
)

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
func (e *CVSS20ReportConfidence) UnmarshalText(data []byte) error {
	s, err := cvss20ReportConfidencePattern(data)
	if err == nil {
		*e = CVSS20ReportConfidence(s)
	}
	return err
}

// CVSS20TargetDistribution represents the targetDistributionType in CVSS20.
type CVSS20TargetDistribution string

const (
	// CVSS20TargetDistributionNone is a constant for "NONE".
	CVSS20TargetDistributionNone CVSS20TargetDistribution = "NONE"
	// CVSS20TargetDistributionLow is a constant for "LOW".
	CVSS20TargetDistributionLow CVSS20TargetDistribution = "LOW"
	// CVSS20TargetDistributionMedium is a constant for "MEDIUM".
	CVSS20TargetDistributionMedium CVSS20TargetDistribution = "MEDIUM"
	// CVSS20TargetDistributionHigh is a constant for "HIGH".
	CVSS20TargetDistributionHigh CVSS20TargetDistribution = "HIGH"
	// CVSS20TargetDistributionNotDefined is a constant for "NOT_DEFINED".
	CVSS20TargetDistributionNotDefined CVSS20TargetDistribution = "NOT_DEFINED"
)

var cvss20TargetDistributionPattern = alternativesUnmarshal(
	string(CVSS20TargetDistributionNone),
	string(CVSS20TargetDistributionLow),
	string(CVSS20TargetDistributionMedium),
	string(CVSS20TargetDistributionHigh),
	string(CVSS20TargetDistributionNotDefined),
)

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
func (e *CVSS20TargetDistribution) UnmarshalText(data []byte) error {
	s, err := cvss20TargetDistributionPattern(data)
	if err == nil {
		*e = CVSS20TargetDistribution(s)
	}
	return err
}
