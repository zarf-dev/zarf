package spdxexp

// The compare methods determine if two ranges are greater than, less than or equal within the same license group.
// NOTE: Ranges are organized into groups (referred to as license groups) of the same base license (e.g. GPL).
//       Groups have sub-groups of license versions (referred to as the range) where each member is considered
//       to be the same version (e.g. {GPL-2.0, GPL-2.0-only}). The sub-groups are in ascending order within
//       the license group, such that the first sub-group is considered to be less than the second sub-group,
//       and so on. (e.g. {{GPL-1.0}, {GPL-2.0, GPL-2.0-only}} implies {GPL-1.0} < {GPL-2.0, GPL-2.0-only}).

// compareGT returns true if the first range is greater than the second range within the same license group; otherwise, false.
func compareGT(first *node, second *node) bool {
	if !first.isLicense() || !second.isLicense() {
		return false
	}
	firstRange := getLicenseRange(*first.license())
	secondRange := getLicenseRange(*second.license())

	if !sameLicenseGroup(firstRange, secondRange) {
		return false
	}
	return firstRange.location[versionGroup] > secondRange.location[versionGroup]
}

// compareLT returns true if the first range is less than the second range within the same license group; otherwise, false.
func compareLT(first *node, second *node) bool {
	if !first.isLicense() || !second.isLicense() {
		return false
	}
	firstRange := getLicenseRange(*first.license())
	secondRange := getLicenseRange(*second.license())

	if !sameLicenseGroup(firstRange, secondRange) {
		return false
	}
	return firstRange.location[versionGroup] < secondRange.location[versionGroup]
}

// compareEQ returns true if the first and second range are the same range within the same license group; otherwise, false.
func compareEQ(first *node, second *node) bool {
	if !first.isLicense() || !second.isLicense() {
		return false
	}
	if first.lic.license == second.lic.license {
		return true
	}

	firstRange := getLicenseRange(*first.license())
	secondRange := getLicenseRange(*second.license())

	if !sameLicenseGroup(firstRange, secondRange) {
		return false
	}
	return firstRange.location[versionGroup] == secondRange.location[versionGroup]
}

// sameLicenseGroup returns false if either license isn't in a range or the two ranges are
// not in the same license group (e.g. group GPL != group Apache); otherwise, true
func sameLicenseGroup(firstRange *licenseRange, secondRange *licenseRange) bool {
	if firstRange == nil || secondRange == nil || firstRange.location[licenseGroup] != secondRange.location[licenseGroup] {
		return false
	}
	return true
}
