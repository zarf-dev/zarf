// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import "path/filepath"

type Images struct {
	Base      string
	Index     string
	OCILayout string
	Blobs     []string
}

func (i *Images) AddBlob(blob string) {
	// TODO: verify sha256 hex
	base := filepath.Join(i.Base, "blobs", "sha256")
	i.Blobs = append(i.Blobs, filepath.Join(base, blob))
}
