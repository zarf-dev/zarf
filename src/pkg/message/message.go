// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package message provides a rich set of functions for displaying messages to the user.
package message

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/pterm/pterm"
)

const (
	// TermWidth sets the width of full width elements like progressbars and headers
	TermWidth = 100
)

var (
	// NoProgress tracks whether spinner/progress bars show updates.
	NoProgress bool
	// RuleLine creates a line of ━ as wide as the terminal
	RuleLine = strings.Repeat("━", TermWidth)
	// OutputWriter provides a default writer to Stdout for user-focused output like tables and yaml
	OutputWriter = os.Stdout
)

func init() {
	InitializePTerm(os.Stderr)
}

// InitializePTerm sets the default styles and output for pterm.
func InitializePTerm(w io.Writer) {
	pterm.ThemeDefault.SuccessMessageStyle = *pterm.NewStyle(pterm.FgLightGreen)
	// Customize default error.
	pterm.Success.Prefix = pterm.Prefix{
		Text:  " ✔",
		Style: pterm.NewStyle(pterm.FgLightGreen),
	}
	pterm.Error.Prefix = pterm.Prefix{
		Text:  "    ERROR:",
		Style: pterm.NewStyle(pterm.BgLightRed, pterm.FgBlack),
	}
	pterm.Info.Prefix = pterm.Prefix{
		Text: " •",
	}

	pterm.SetDefaultOutput(w)
}

// ColorEnabled returns true if color printing is enabled.
func ColorEnabled() bool {
	return pterm.PrintColor
}

// HorizontalRule prints a white horizontal rule to separate the terminal
func HorizontalRule() {
	pterm.Println()
	pterm.Println(RuleLine)
}

// Table prints a padded table containing the specified header and data to writer.
func Table(writer io.Writer, header []string, data [][]string) {
	fmt.Fprintln(writer)

	// To avoid side effects make copies of the header and data before adding padding
	headerCopy := make([]string, len(header))
	copy(headerCopy, header)
	dataCopy := make([][]string, len(data))
	copy(dataCopy, data)
	if len(headerCopy) > 0 {
		headerCopy[0] = fmt.Sprintf("     %s", headerCopy[0])
	}

	table := pterm.TableData{
		headerCopy,
	}

	for _, row := range dataCopy {
		if len(row) > 0 {
			row[0] = fmt.Sprintf("     %s", row[0])
		}
		table = append(table, pterm.TableData{row}...)
	}

	_ = pterm.DefaultTable.WithWriter(writer).WithHasHeader().WithData(table).Render() //nolint:errcheck // Table output is best effort, matching the other message printers.
}
