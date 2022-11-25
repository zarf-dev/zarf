// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package message provides a rich set of functions for displaying messages to the user.
package message

import (
	"fmt"

	"github.com/pterm/pterm"
)

var activeSpinner *Spinner

// Spinner wraps the pterm SpinnerPrinter.
type Spinner struct {
	spinner   *pterm.SpinnerPrinter
	startText string
}

// NewProgressSpinner creates and starts a new ProgressSpinner.
func NewProgressSpinner(format string, a ...any) *Spinner {
	if activeSpinner != nil {
		Debug("Active spinner already exists")
		return activeSpinner
	}

	var spinner *pterm.SpinnerPrinter
	text := fmt.Sprintf(format, a...)
	if NoProgress {
		Info(text)
	} else {
		spinner, _ = pterm.DefaultSpinner.
			WithRemoveWhenDone(false).
			// Src: https://github.com/gernest/wow/blob/master/spin/spinners.go#L335
			WithSequence(`  ⠋ `, `  ⠙ `, `  ⠹ `, `  ⠸ `, `  ⠼ `, `  ⠴ `, `  ⠦ `, `  ⠧ `, `  ⠇ `, `  ⠏ `).
			Start(text)
	}

	activeSpinner = &Spinner{
		spinner:   spinner,
		startText: text,
	}

	return activeSpinner
}

// Write writes a byte array at the debug log level to stdout and returns the number of bytes written.
func (p *Spinner) Write(text []byte) (int, error) {
	size := len(text)
	if NoProgress {
		return size, nil
	}
	Debug(string(text))
	return len(text), nil
}

// Updatef updates the spinner text.
func (p *Spinner) Updatef(format string, a ...any) {
	if NoProgress {
		return
	}

	text := fmt.Sprintf(format, a...)
	p.spinner.UpdateText(text)
}

// Debugf Updates the spinner text with a formatted string input.
func (p *Spinner) Debugf(format string, a ...any) {
	if logLevel >= DebugLevel {
		text := fmt.Sprintf("Debug: "+format, a)
		if NoProgress {
			Debug(text)
		} else {
			p.spinner.UpdateText(text)
		}
	}
}

// Stop stops the spinner.
func (p *Spinner) Stop() {
	if p.spinner != nil && p.spinner.IsActive {
		_ = p.spinner.Stop()
	}
	activeSpinner = nil
}

/*
Success prints a success message and stops the spinner.

The success message is the same as the text used when the spinner was created.
*/
func (p *Spinner) Success() {
	p.Successf(p.startText)
}

// Successf prints a formatted success message and stops the spinner.
func (p *Spinner) Successf(format string, a ...any) {
	text := fmt.Sprintf(format, a...)
	if p.spinner != nil {
		p.spinner.Success(text)
		activeSpinner = nil
	} else {
		Info(text)
	}
}

// Warnf updates the spinner with a formatted warning message.
func (p *Spinner) Warnf(format string, a ...any) {
	text := fmt.Sprintf(format, a...)
	if p.spinner != nil {
		p.spinner.Warning(text)
	} else {
		Warn(text)
	}
}

// Errorf updates the spinner with a formatted error message and logs the error at the debug level.
func (p *Spinner) Errorf(err error, format string, a ...any) {
	p.Warnf(format, a...)
	Debug(err)
}

// Fatal stops the spinner, prints a fatal error message, and exits the program.
func (p *Spinner) Fatal(err error) {
	p.Fatalf(err, p.startText)
}

// Fatal stops the spinner, prints a formatted fatal error message, and exits the program.
func (p *Spinner) Fatalf(err error, format string, a ...any) {
	if p.spinner != nil {
		p.spinner.RemoveWhenDone = true
		_ = p.spinner.Stop()
		activeSpinner = nil
	}
	Fatalf(err, format, a...)
}
