// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package message provides a rich set of functions for displaying messages to the user.
package message

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/pterm/pterm"
)

var activeSpinner *Spinner

var sequence = []string{`  ⠋ `, `  ⠙ `, `  ⠹ `, `  ⠸ `, `  ⠼ `, `  ⠴ `, `  ⠦ `, `  ⠧ `, `  ⠇ `, `  ⠏ `}

// Spinner is a wrapper around pterm.SpinnerPrinter.
type Spinner struct {
	spinner        *pterm.SpinnerPrinter
	startText      string
	termWidth      int
	preserveWrites bool
}

// NewProgressSpinner creates a new progress spinner.
func NewProgressSpinner(format string, a ...any) *Spinner {
	if activeSpinner != nil {
		Debug("Active spinner already exists")
		return activeSpinner
	}

	var spinner *pterm.SpinnerPrinter
	text := pterm.Sprintf(format, a...)
	if NoProgress {
		Info(text)
	} else {
		spinner, _ = pterm.DefaultSpinner.
			WithRemoveWhenDone(false).
			// Src: https://github.com/gernest/wow/blob/master/spin/spinners.go#L335
			WithSequence(sequence...).
			Start(text)
	}

	activeSpinner = &Spinner{
		spinner:   spinner,
		startText: text,
		termWidth: pterm.GetTerminalWidth(),
	}

	return activeSpinner
}

// EnablePreserveWrites enables preserving writes to the terminal.
func (p *Spinner) EnablePreserveWrites() {
	p.preserveWrites = true
}

// DisablePreserveWrites disables preserving writes to the terminal.
func (p *Spinner) DisablePreserveWrites() {
	p.preserveWrites = false
}

// Write the given text to the spinner.
func (p *Spinner) Write(raw []byte) (int, error) {
	size := len(raw)
	if NoProgress {
		pterm.Printfln("     %s", string(raw))
		return size, nil
	}

	// Split the text into lines and update the spinner for each line.
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		// Only be fancy if preserve writes is enabled.
		if p.preserveWrites {
			text := pterm.Sprintf("     %s", scanner.Text())
			pterm.Fprinto(p.spinner.Writer, strings.Repeat(" ", p.termWidth))
			pterm.Fprintln(p.spinner.Writer, text)
		} else {
			// Otherwise just update the spinner text.
			p.spinner.UpdateText(scanner.Text())
		}
	}

	return size, nil
}

// Updatef updates the spinner text.
func (p *Spinner) Updatef(format string, a ...any) {
	if NoProgress {
		return
	}

	text := pterm.Sprintf(format, a...)
	p.spinner.UpdateText(text)
}

// Stop the spinner.
func (p *Spinner) Stop() {
	if p.spinner != nil && p.spinner.IsActive {
		_ = p.spinner.Stop()
	}
	activeSpinner = nil
}

// Success prints a success message and stops the spinner.
func (p *Spinner) Success() {
	p.Successf(p.startText)
}

// Successf prints a success message with the spinner and stops it.
func (p *Spinner) Successf(format string, a ...any) {
	text := pterm.Sprintf(format, a...)
	if p.spinner != nil {
		p.spinner.Success(text)
	} else {
		Info(text)
	}
	p.Stop()
}

// Warnf prints a warning message with the spinner.
func (p *Spinner) Warnf(format string, a ...any) {
	text := pterm.Sprintf(format, a...)
	if p.spinner != nil {
		p.spinner.Warning(text)
	} else {
		Warn(text)
	}
}

// Errorf prints an error message with the spinner.
func (p *Spinner) Errorf(err error, format string, a ...any) {
	p.Warnf(format, a...)
	Debug(err)
}

// Fatal calls message.Fatalf with the given error.
func (p *Spinner) Fatal(err error) {
	p.Fatalf(err, p.startText)
}

// Fatalf calls message.Fatalf with the given error and format.
func (p *Spinner) Fatalf(err error, format string, a ...any) {
	if p.spinner != nil {
		p.spinner.RemoveWhenDone = true
		_ = p.spinner.Stop()
		activeSpinner = nil
	}
	Fatalf(err, format, a...)
}

type MultiSpinner struct {
	area          *pterm.AreaPrinter
	startedAt     time.Time
	rows          []MultiSpinnerRow
	SuccessStatus string
	ErrorStatus   string
}

type MultiSpinnerRow struct {
	Status string
	Text   string
}

var activeMultiSpinner *MultiSpinner

func NewMultiSpinner() *MultiSpinner {
	if activeMultiSpinner != nil {
		Debug("Active multi spinner already exists")
		return activeMultiSpinner
	}
	var area *pterm.AreaPrinter
	area, _ = pterm.DefaultArea.
		WithRemoveWhenDone(false).Start()
	return &MultiSpinner{
		area:          area,
		SuccessStatus: pterm.FgLightGreen.Sprint("  ✔ "),
		ErrorStatus:   pterm.FgLightRed.Sprint("  ✖ "),
	}
}

func (m MultiSpinner) Start() *MultiSpinner {
	m.startedAt = time.Now()
	activeMultiSpinner = &m
	delay := pterm.DefaultSpinner.Delay

	if NoProgress {
		return &m
	}

	go func() {
		for activeMultiSpinner != nil {
			text := m.renderText()
			m.area.Update(text)
			time.Sleep(delay)
		}
	}()
	return &m
}

func (m *MultiSpinner) renderText() string {
	var text string
	for idx, row := range m.rows {
		switch row.Status {
		case "":
			m.rows[idx].Status = sequence[0]
		default:
			for i, s := range sequence {
				if s == row.Status {
					m.rows[idx].Status = sequence[(i+1)%len(sequence)]
					break
				}
			}
		}
		var timer string
		if row.Status != m.SuccessStatus && row.Status != m.ErrorStatus {
			timer = pterm.ThemeDefault.TimerStyle.Sprint(" (" + time.Since(m.startedAt).Round(time.Second).String() + ")")
		}
		text += fmt.Sprintf("%s %s%s\n", row.Status, row.Text, timer)
	}
	return text
}

func (m *MultiSpinner) Stop() {
	if m.area != nil && activeMultiSpinner != nil && !NoProgress {
		m.area.Update(m.renderText())
		_ = m.area.Stop()
	}
	activeMultiSpinner = nil
}

func (m *MultiSpinner) Update(rows []MultiSpinnerRow) {
	if NoProgress {
		for i, row := range m.rows {
			if row.Status != rows[i].Status {
				switch rows[i].Status {
				case m.SuccessStatus:
					Successf(rows[i].Text)
				case m.ErrorStatus:
					Warn(rows[i].Text)
				}
			}
		}
		for i := len(m.rows); i < len(rows); i++ {
			Info(rows[i].Text)
		}
	}
	m.rows = rows
}

func NewMultiSpinnerRow(text string) MultiSpinnerRow {
	return MultiSpinnerRow{
		Text:   text,
		Status: "",
	}
}

func (m *MultiSpinner) RowSuccess(index int) {
	m.rows[index].Status = m.SuccessStatus
	m.rows[index].Text = pterm.FgLightGreen.Sprint(m.rows[index].Text)
	if NoProgress {
		Successf(m.rows[index].Text)
	}
}

func (m *MultiSpinner) RowError(index int) {
	m.rows[index].Status = m.ErrorStatus
	m.rows[index].Text = pterm.FgLightRed.Sprint(m.rows[index].Text)
	if NoProgress {
		Warnf(m.rows[index].Text)
	}
}

func (m *MultiSpinner) GetContent() []MultiSpinnerRow {
	return m.rows
}