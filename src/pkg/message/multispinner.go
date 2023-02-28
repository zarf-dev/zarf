// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package message provides a rich set of functions for displaying messages to the user.
package message

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/pterm/pterm"
)

// MultiSpinner is a wrapper around pterm.AreaPrinter and structures around rows to track the state of each spinner.
type MultiSpinner struct {
	area      *pterm.AreaPrinter
	startedAt time.Time
	rows      []MultiSpinnerRow
	mutex     sync.Mutex
}

// MultiSpinnerRow is a row in a multispinner.
type MultiSpinnerRow struct {
	Status string
	Text   string
}

var activeMultiSpinner *MultiSpinner

// RowStatusSuccess is the success status for a row.
var RowStatusSuccess = pterm.FgLightGreen.Sprint("  ✔ ")

// RowStatusError is the error status for a row.
var RowStatusError = pterm.FgLightRed.Sprint("  ✖ ")

// NewMultiSpinner creates a new multispinner instance if one does not exist.
func NewMultiSpinner() *MultiSpinner {
	if activeMultiSpinner != nil {
		return activeMultiSpinner
	}
	area, _ := pterm.DefaultArea.
		WithRemoveWhenDone(true).Start()
	m := &MultiSpinner{
		area:      area,
		startedAt: time.Now(),
	}
	activeMultiSpinner = m
	delay := pterm.DefaultSpinner.Delay
	if NoProgress {
		_ = activeMultiSpinner.area.Stop()
		activeMultiSpinner = nil
		return m
	}
	go func() {
		for activeMultiSpinner != nil {
			m.mutex.Lock()

			text := m.renderText()
			m.area.Update(text)
			time.Sleep(delay)

			m.mutex.Unlock()
		}
	}()
	return m
}

// renderText renders the rows into a string to be used by pterm.AreaPrinter.
func (m *MultiSpinner) renderText() string {
	var outputRows []string
	for idx, row := range m.rows {
		if row.Status != RowStatusSuccess && row.Status != RowStatusError {
			for i, s := range sequence {
				if s == row.Status {
					m.rows[idx].Status = sequence[(i+1)%len(sequence)]
					break
				}
			}
			timer := pterm.ThemeDefault.TimerStyle.Sprint(" (" + time.Since(m.startedAt).Round(time.Second).String() + ")")
			outputRows = append(outputRows, fmt.Sprintf("%s %s%s", row.Status, row.Text, timer))
		}
	}
	return strings.Join(outputRows, "\n")
}

// Stop stops the multispinner.
func (m *MultiSpinner) Stop() {
	if m.area != nil && activeMultiSpinner != nil && !NoProgress {
		m.mutex.Lock()

		m.area.Update(m.renderText())
		_ = m.area.Stop()

		m.mutex.Unlock()
	}
	activeMultiSpinner = nil
}

// NewMultiSpinnerRow creates a new row for a multispinner but does not add it to current rows, use Update.
func NewMultiSpinnerRow(text string) MultiSpinnerRow {
	return MultiSpinnerRow{
		Text:   text,
		Status: sequence[0],
	}
}

func (m *MultiSpinner) AddRow(row MultiSpinnerRow) {
	m.mutex.Lock()

	m.rows = append(m.rows, row)

	m.mutex.Unlock()
}

// RowSuccess sets the status of a row to success.
func (m *MultiSpinner) RowSuccess(message string) {
	m.mutex.Lock()

	for idx := range m.rows {
		if m.rows[idx].Text == message {
			m.rows[idx].Status = RowStatusSuccess
		}
	}
	Successf("%s", message)

	m.mutex.Unlock()
}

// RowError sets the status of a row to error.
func (m *MultiSpinner) RowError(message string) {
	m.mutex.Lock()

	for idx := range m.rows {
		if m.rows[idx].Text == message {
			m.rows[idx].Status = RowStatusError
		}
	}
	Warnf("%s", message)

	m.mutex.Unlock()
}

// GetContent returns the current rows of the multispinner.
func (m *MultiSpinner) GetContent() []MultiSpinnerRow {
	return m.rows
}
