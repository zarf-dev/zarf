// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later

package writer

import (
	"fmt"
	"io"

	spdx "github.com/spdx/tools-golang/spdx/v2/v2_1"
)

func renderOtherLicense(ol *spdx.OtherLicense, w io.Writer) error {
	if ol.LicenseIdentifier != "" {
		fmt.Fprintf(w, "LicenseID: %s\n", ol.LicenseIdentifier)
	}
	if ol.ExtractedText != "" {
		fmt.Fprintf(w, "ExtractedText: %s\n", textify(ol.ExtractedText))
	}
	if ol.LicenseName != "" {
		fmt.Fprintf(w, "LicenseName: %s\n", ol.LicenseName)
	}
	for _, s := range ol.LicenseCrossReferences {
		fmt.Fprintf(w, "LicenseCrossReference: %s\n", s)
	}
	if ol.LicenseComment != "" {
		fmt.Fprintf(w, "LicenseComment: %s\n", textify(ol.LicenseComment))
	}

	fmt.Fprintf(w, "\n")

	return nil
}
