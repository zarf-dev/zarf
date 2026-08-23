// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later

package writer

import (
	"fmt"
	"strings"
)

func textify(s string) string {
	if strings.Contains(s, "\n") {
		return fmt.Sprintf("<text>%s</text>", s)
	}

	return s
}
