// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package message provides a rich set of functions for displaying messages to the user.
package message

import (
	"fmt"
	"strings"
	"time"

	"github.com/pterm/pterm"
)

// MultiSpinner is a wrapper around pterm.AreaPrinter and structures around rows to track the state of each spinner.
type MultiSpinner struct {
	area      *pterm.AreaPrinter
	startedAt time.Time
	rows      []MultiSpinnerRow
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
		WithRemoveWhenDone(false).Start()
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
			text := m.renderText()
			m.area.Update(text)
			time.Sleep(delay)
		}
	}()
	return m
}

// renderText renders the rows into a string to be used by pterm.AreaPrinter.
func (m *MultiSpinner) renderText() string {
	var outputRows []string
	for idx, row := range m.rows {
		for i, s := range sequence {
			if s == row.Status {
				m.rows[idx].Status = sequence[(i+1)%len(sequence)]
				break
			}
		}
		var timer string
		if row.Status != RowStatusSuccess && row.Status != RowStatusError {
			timer = pterm.ThemeDefault.TimerStyle.Sprint(" (" + time.Since(m.startedAt).Round(time.Second).String() + ")")
		}
		outputRows = append(outputRows, fmt.Sprintf("%s %s%s", row.Status, row.Text, timer))
	}
	return strings.Join(outputRows, "\n")
}

// Stop stops the multispinner.
func (m *MultiSpinner) Stop() {
	if m.area != nil && activeMultiSpinner != nil && !NoProgress {
		m.area.Update(m.renderText())
		_ = m.area.Stop()
	}
	activeMultiSpinner = nil
}

// Update updates the rows of the multispinner, re-renders are handled by the goroutine.
func (m *MultiSpinner) Update(rows []MultiSpinnerRow) {
	if NoProgress {
		for i, row := range m.rows {
			if row.Status != rows[i].Status {
				switch rows[i].Status {
				case RowStatusError:
					Successf(rows[i].Text)
				case RowStatusError:
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

// NewMultiSpinnerRow creates a new row for a multispinner but does not add it to current rows, use Update.
func NewMultiSpinnerRow(text string) MultiSpinnerRow {
	return MultiSpinnerRow{
		Text:   text,
		Status: sequence[0],
	}
}

// RowSuccess sets the status of a row to success.
func (m *MultiSpinner) RowSuccess(index int) {
	m.rows[index].Status = RowStatusSuccess
	m.rows[index].Text = pterm.FgLightGreen.Sprint(m.rows[index].Text)
	if NoProgress {
		Successf(m.rows[index].Text)
	}
}

// RowError sets the status of a row to error.
func (m *MultiSpinner) RowError(index int) {
	m.rows[index].Status = RowStatusError
	m.rows[index].Text = pterm.FgLightRed.Sprint(m.rows[index].Text)
	if NoProgress {
		Warnf(m.rows[index].Text)
	}
}

// GetContent returns the current rows of the multispinner.
func (m *MultiSpinner) GetContent() []MultiSpinnerRow {
	return m.rows
}
