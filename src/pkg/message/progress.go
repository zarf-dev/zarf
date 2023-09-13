// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package message provides a rich set of functions for displaying messages to the user.
package message

import (
	"github.com/pterm/pterm"
)

const padding = "    "

// ProgressBar is a struct used to drive a pterm ProgressbarPrinter.
type ProgressBar struct {
	progress  *pterm.ProgressbarPrinter
	startText string
}

// NewProgressBar creates a new ProgressBar instance from a total value and a format.
func NewProgressBar(total int64, text string) *ProgressBar {
	var progress *pterm.ProgressbarPrinter
	if NoProgress {
		Info(text)
	} else {
		progress, _ = pterm.DefaultProgressbar.
			WithTotal(int(total)).
			WithShowCount(false).
			WithTitle(padding + text).
			WithRemoveWhenDone(true).
			WithMaxWidth(TermWidth).
			Start()
	}

	return &ProgressBar{
		progress:  progress,
		startText: text,
	}
}

// Update updates the ProgressBar with completed progress and new text.
func (p *ProgressBar) Update(complete int64, text string) {
	if NoProgress {
		debugPrinter(2, text)
		return
	}
	p.progress.UpdateTitle(padding + text)
	chunk := int(complete) - p.progress.Current
	p.Add(chunk)
}

// UpdateTitle updates the ProgressBar with new text.
func (p *ProgressBar) UpdateTitle(text string) {
	if NoProgress {
		debugPrinter(2, text)
		return
	}
	p.progress.UpdateTitle(padding + text)
}

// Add updates the ProgressBar with completed progress.
func (p *ProgressBar) Add(n int) {
	if p.progress != nil {
		if p.progress.Current+n >= p.progress.Total {
			// @RAZZLE TODO: This is a hack to prevent the progress bar from going over 100% and causing TUI ugliness.
			overflow := p.progress.Current + n - p.progress.Total
			p.progress.Total += overflow + 1
		}
		p.progress.Add(n)
	}
}

// Write updates the ProgressBar with the number of bytes in a buffer as the completed progress.
func (p *ProgressBar) Write(data []byte) (int, error) {
	n := len(data)
	if p.progress != nil {
		p.Add(n)
	}
	return n, nil
}

// Successf marks the ProgressBar as successful in the CLI.
func (p *ProgressBar) Successf(format string, a ...any) {
	p.Stop()
	pterm.Success.Printfln(format, a...)
}

// Stop stops the ProgressBar from continuing.
func (p *ProgressBar) Stop() {
	if p.progress != nil {
		_, _ = p.progress.Stop()
	}
}

// Errorf marks the ProgressBar as failed in the CLI.
func (p *ProgressBar) Errorf(err error, format string, a ...any) {
	p.Stop()
	WarnErrf(err, format, a...)
}
