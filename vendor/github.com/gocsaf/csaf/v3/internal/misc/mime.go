// This file is Free Software under the Apache-2.0 License
// without warranty, see README.md and LICENSES/Apache-2.0.txt for details.
//
// SPDX-License-Identifier: Apache-2.0
//
// SPDX-FileCopyrightText: 2023 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
// Software-Engineering: 2023 Intevation GmbH <https://intevation.de>

package misc //revive:disable-line:var-naming

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
	"strings"
)

var escapeQuotes = strings.NewReplacer("\\", "\\\\", `"`, "\\\"").Replace

// CreateFormFile creates an [io.Writer] like [mime/multipart.Writer.CreateFromFile].
// This version allows to set the mime type, too.
func CreateFormFile(w *multipart.Writer, fieldname, filename, mimeType string) (io.Writer, error) {
	// Source: https://cs.opensource.google/go/go/+/refs/tags/go1.20:src/mime/multipart/writer.go;l=140
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
			escapeQuotes(fieldname), escapeQuotes(filename)))
	h.Set("Content-Type", mimeType)
	return w.CreatePart(h)
}
