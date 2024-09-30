// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package message provides a rich set of functions for displaying messages to the user.
package message

import (
	"fmt"
	"os"

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
	var err error
	if NoProgress {
		Info(text)
	} else {
		progress, err = pterm.DefaultProgressbar.
			WithTotal(int(total)).
			WithShowCount(false).
			WithTitle(padding + text).
			WithRemoveWhenDone(true).
			WithMaxWidth(TermWidth).
			WithWriter(os.Stderr).
			Start()
		if err != nil {
			WarnErr(err, "Unable to create default progressbar")
		}
	}

	return &ProgressBar{
		progress:  progress,
		startText: text,
	}
}

// Updatef updates the ProgressBar with new text.
func (p *ProgressBar) Updatef(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	if NoProgress {
		debugPrinter(2, msg)
		return
	}
	p.progress.UpdateTitle(padding + msg)
}

// Failf marks the ProgressBar as failed in the CLI.
func (p *ProgressBar) Failf(format string, a ...any) {
	err := p.Close()
	if err != nil {
		Debug("unable to close failed progressbar", "error", err)
	}
	Warnf(format, a...)
}

// Close stops the ProgressBar from continuing.
func (p *ProgressBar) Close() error {
	if p.progress == nil {
		return nil
	}
	_, err := p.progress.Stop()
	if err != nil {
		return err
	}
	return nil
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
	err := p.Close()
	if err != nil {
		Debug("unable to close successful progressbar", "error", err)
	}
	pterm.Success.Printfln(format, a...)
}

// GetCurrent returns the current total
func (p *ProgressBar) GetCurrent() int {
	if p.progress != nil {
		return p.progress.Current
	}
	return -1
}
