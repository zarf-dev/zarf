// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package message provides a rich set of functions for displaying messages to the user.
package message

import (
	"fmt"

	"github.com/defenseunicorns/zarf/src/types"
	"github.com/pterm/pterm"
)

// PrintConnectStringTable prints a table of connect strings.
func PrintConnectStringTable(connectStrings types.ConnectStrings) {
	Debugf("message.PrintConnectStringTable(%#v)", connectStrings)

	if len(connectStrings) > 0 {
		list := pterm.TableData{{"     Connect Command", "Description"}}
		// Loop over each connectStrings and convert to pterm.TableData
		for name, connect := range connectStrings {
			name = fmt.Sprintf("     zarf connect %s", name)
			list = append(list, []string{name, connect.Description})
		}

		// Create the table output with the data
		_ = pterm.DefaultTable.WithHasHeader().WithData(list).Render()
	}
}
