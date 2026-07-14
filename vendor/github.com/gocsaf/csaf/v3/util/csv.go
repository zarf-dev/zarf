// This file is Free Software under the Apache-2.0 License
// without warranty, see README.md and LICENSES/Apache-2.0.txt for details.
//
// SPDX-License-Identifier: Apache-2.0
//
// SPDX-FileCopyrightText: 2022 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
// Software-Engineering: 2022 Intevation GmbH <https://intevation.de>

package util //revive:disable-line:var-naming

import (
	"bufio"
	"io"
	"strings"
)

// FullyQuotedCSVWriter implements a CSV writer
// which puts each field in double quotes (").
type FullyQuotedCSVWriter struct {
	// Comma is the separator between fields. Defaults to ','.
	Comma rune
	// UseCRLF indicates if "\r\n" should be used for line separation.
	UseCRLF bool
	w       *bufio.Writer
}

// NewFullyQuotedCSWWriter returns a new writer that writes to w.
func NewFullyQuotedCSWWriter(w io.Writer) *FullyQuotedCSVWriter {
	return &FullyQuotedCSVWriter{
		Comma: ',',
		w:     bufio.NewWriter(w),
	}
}

// Write writes a single CSV record to w along with any necessary quoting.
// A record is a slice of strings with each string being one field.
// Writes are buffered, so Flush must eventually be called to ensure
// that the record is written to the underlying io.Writer.
func (fqcw *FullyQuotedCSVWriter) Write(record []string) error {

	for i, field := range record {
		if i > 0 {
			fqcw.w.WriteRune(fqcw.Comma)
		}
		fqcw.w.WriteByte('"')
		if !fqcw.UseCRLF {
			field = strings.ReplaceAll(field, "\r\n", "\n")
		}
		fqcw.w.WriteString(strings.ReplaceAll(field, `"`, `""`))
		fqcw.w.WriteByte('"')
	}
	var err error
	if fqcw.UseCRLF {
		_, err = fqcw.w.WriteString("\r\n")
	} else {
		err = fqcw.w.WriteByte('\n')
	}
	return err
}

// Flush writes any buffered data to the underlying io.Writer.
// To check if an error occurred during the Flush, call Error.
func (fqcw *FullyQuotedCSVWriter) Flush() {
	fqcw.w.Flush()
}

// Error reports any error that has occurred during a previous Write or Flush.
func (fqcw *FullyQuotedCSVWriter) Error() error {
	_, err := fqcw.w.Write(nil)
	return err
}
