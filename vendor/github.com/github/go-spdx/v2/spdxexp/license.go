package spdxexp

import (
	"strings"

	"github.com/github/go-spdx/v2/spdxexp/spdxlicenses"
)

// activeLicense returns true if the id is an active license.
func activeLicense(id string) (bool, string) {
	return spdxlicenses.IsActiveLicense(id)
}

// ActiveLicense returns true if the id is an active license.
func ActiveLicense(id string) (bool, string) {
	return activeLicense(id)
}

// deprecatedLicense returns true if the id is a deprecated license.
func deprecatedLicense(id string) (bool, string) {
	return spdxlicenses.IsDeprecatedLicense(id)
}

// exceptionLicense returns true if the id is an exception license.
func exceptionLicense(id string) (bool, string) {
	return spdxlicenses.IsException(id)
}

const (
	licenseGroup uint8 = iota
	versionGroup
	licenseIndex
)

type licenseRange struct {
	licenses []string
	location map[uint8]int // licenseGroup, versionGroup, licenseIndex
}

// getLicenseRange returns a range of licenses from licenseRanges
func getLicenseRange(id string) *licenseRange {
	simpleID := simplifyLicense(id)
	allRanges := spdxlicenses.LicenseRanges()
	for i, licenseGrp := range allRanges {
		for j, versionGrp := range licenseGrp {
			for k, license := range versionGrp {
				if simpleID == license {
					location := map[uint8]int{
						licenseGroup: i,
						versionGroup: j,
						licenseIndex: k,
					}
					return &licenseRange{
						licenses: versionGrp,
						location: location,
					}
				}
			}
		}
	}
	return nil
}

func simplifyLicense(id string) string {
	if strings.HasSuffix(id, "-or-later") {
		return id[0 : len(id)-9]
	}
	return id
}
