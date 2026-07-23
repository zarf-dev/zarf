// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later

package reader

import (
	"fmt"
	"strings"
)

func (parser *tvParser) parsePairForRelationship(tag string, value string) error {
	if parser.rln == nil {
		return fmt.Errorf("no relationship struct created in parser rln pointer")
	}

	if tag == "Relationship" {
		// parse the value to see if it's a valid relationship format
		sp := strings.SplitN(value, " ", -1)

		// filter out any purely-whitespace items
		var rp []string
		for _, v := range sp {
			v = strings.TrimSpace(v)
			if v != "" {
				rp = append(rp, v)
			}
		}

		if len(rp) != 3 {
			return fmt.Errorf("invalid relationship format for %s", value)
		}

		aID, err := extractDocElementID(strings.TrimSpace(rp[0]))
		if err != nil {
			return err
		}
		parser.rln.RefA = aID
		parser.rln.Relationship = strings.TrimSpace(rp[1])
		bID, err := extractDocElementID(strings.TrimSpace(rp[2]))
		if err != nil {
			return err
		}
		parser.rln.RefB = bID
		return nil
	}

	if tag == "RelationshipComment" {
		parser.rln.RelationshipComment = value
		return nil
	}

	return fmt.Errorf("received unknown tag %v in Relationship section", tag)
}
