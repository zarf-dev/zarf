// This file is Free Software under the Apache-2.0 License
// without warranty, see README.md and LICENSES/Apache-2.0.txt for details.
//
// SPDX-License-Identifier: Apache-2.0
//
// SPDX-FileCopyrightText: 2022 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
// Software-Engineering: 2022 Intevation GmbH <https://intevation.de>

package csaf

import (
	"bufio"
	"io"
	"strings"
)

// ExtractProviderURL extracts URLs of provider metadata.
// If all is true all URLs are returned. Otherwise only the first is returned.
func ExtractProviderURL(r io.Reader, all bool) ([]string, error) {
	const csaf = "CSAF:"

	var urls []string

	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, csaf) {
			urls = append(urls, strings.TrimSpace(line[len(csaf):]))
			if !all {
				return urls, nil
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return urls, nil
}

// CollectProductIdentificationHelpers returns a slice of all ProductIdentificationHelper
// for a given ProductID.
func (pt *ProductTree) CollectProductIdentificationHelpers(id ProductID) []*ProductIdentificationHelper {
	var helpers []*ProductIdentificationHelper
	pt.FindProductIdentificationHelpers(
		id, func(helper *ProductIdentificationHelper) {
			helpers = append(helpers, helper)
		})
	return helpers
}

// FindProductIdentificationHelpers calls visit on all ProductIdentificationHelper
// for a given ProductID by iterating over all full product names and branches
// recursively available in the ProductTree.
func (pt *ProductTree) FindProductIdentificationHelpers(
	id ProductID,
	visit func(*ProductIdentificationHelper),
) {
	// Iterate over all full product names
	if fpns := pt.FullProductNames; fpns != nil {
		for _, fpn := range *fpns {
			if fpn != nil &&
				fpn.ProductID != nil && *fpn.ProductID == id &&
				fpn.ProductIdentificationHelper != nil {
				visit(fpn.ProductIdentificationHelper)
			}
		}
	}

	// Iterate over branches recursively
	var recBranch func(b *Branch)
	recBranch = func(b *Branch) {
		if b == nil {
			return
		}
		if fpn := b.Product; fpn != nil &&
			fpn.ProductID != nil && *fpn.ProductID == id &&
			fpn.ProductIdentificationHelper != nil {
			visit(fpn.ProductIdentificationHelper)
		}
		for _, c := range b.Branches {
			recBranch(c)
		}
	}
	for _, b := range pt.Branches {
		recBranch(b)
	}

	// Iterate over relationships
	if rels := pt.RelationShips; rels != nil {
		for _, rel := range *rels {
			if rel != nil {
				if fpn := rel.FullProductName; fpn != nil && fpn.ProductID != nil &&
					*fpn.ProductID == id && fpn.ProductIdentificationHelper != nil {
					visit(fpn.ProductIdentificationHelper)
				}
			}
		}
	}
}
