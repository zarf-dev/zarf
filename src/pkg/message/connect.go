// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package message provides a rich set of functions for displaying messages to the user.
package message

import (
	"fmt"
	"os"

	"github.com/zarf-dev/zarf/src/types"
)

// PrintConnectStringTable prints a table of connect strings.
func PrintConnectStringTable(connectStrings types.ConnectStrings) {
	if len(connectStrings) > 0 {
		connectData := [][]string{}
		// Loop over each connectStrings and convert to a string matrix
		for name, connect := range connectStrings {
			name = fmt.Sprintf("zarf connect %s", name)
			connectData = append(connectData, []string{name, connect.Description})
		}

		// HACK(mkcp): Setting a PTerm global during runtime isn't ideal or thread-safe. However, it lets us render
		// even when message is disabled.
		lastWriter := *PTermWriter.Load()
		InitializePTerm(os.Stdout)
		// Create the table output with the data
		header := []string{"Connect Command", "Description"}
		Table(header, connectData)
		InitializePTerm(lastWriter)
	}
}
