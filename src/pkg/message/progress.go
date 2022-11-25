// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package message provides a rich set of functions for displaying messages to the user.
package message

import (
	"fmt"

	"github.com/pterm/pterm"
)

// ProgressBar wraps the pterm progress bar.
type ProgressBar struct {
	progress  *pterm.ProgressbarPrinter
	startText string
}

// NewProgressBar starts a new progress bar.
func NewProgressBar(total int64, format string, a ...any) *ProgressBar {
	var progress *pterm.ProgressbarPrinter
	text := fmt.Sprintf("     "+format, a...)
	if NoProgress {
		Info(text)
	} else {
		progress, _ = pterm.DefaultProgressbar.
			WithTotal(int(total)).
			WithShowCount(false).
			WithTitle(text).
			WithRemoveWhenDone(true).
			Start()
	}

	return &ProgressBar{
		progress:  progress,
		startText: text,
	}
}

// Update increments the progress bar to match the provided completed count.
func (p *ProgressBar) Update(complete int64, text string) {
	if NoProgress {
		return
	}
	p.progress.UpdateTitle("     " + text)
	chunk := int(complete) - p.progress.Current
	p.progress.Add(chunk)
}

// Write prints the provided data to the progress bar.
func (p *ProgressBar) Write(data []byte) (int, error) {
	n := len(data)
	if p.progress != nil {
		p.progress.Add(n)
	}
	return n, nil
}

// Success prints a success message and stops the progress bar.
func (p *ProgressBar) Success(text string, a ...any) {
	p.Stop()
	pterm.Success.Printfln(text, a...)
}

// Stop stops the progress bar.
func (p *ProgressBar) Stop() {
	if p.progress != nil {
		_, _ = p.progress.Stop()
	}
}

// Fatalf stops the progress bar, prints a fatal error message, and exits the program.
func (p *ProgressBar) Fatalf(err error, format string, a ...any) {
	p.Stop()
	Fatalf(err, format, a...)
}
