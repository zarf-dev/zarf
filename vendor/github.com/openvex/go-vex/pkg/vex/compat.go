/*
Copyright 2023 The OpenVEX Authors
SPDX-License-Identifier: Apache-2.0
*/

package vex

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

type legacyParser func([]byte) (*VEX, error)

// getLegacyVersionParser returns a parser that can read older OpenVEX formats. The
// project will have a version skew policy and try to support older versions
// up to a point. If a version is not supported, this function returns nil.
func getLegacyVersionParser(version string) legacyParser {
	switch version {
	case "v0.0.1":
		return parse001
	default:
		return nil
	}
}

var parse001 = func(data []byte) (*VEX, error) {
	oldVex := &vex001{}

	if err := json.Unmarshal(data, oldVex); err != nil {
		return nil, fmt.Errorf(
			"decoding OpenVEX v0.0.1 in compatibility mode: %w", err,
		)
	}

	newVex := New()

	newVex.Timestamp = oldVex.Timestamp
	newVex.Author = oldVex.Author
	newVex.AuthorRole = oldVex.AuthorRole
	newVex.ID = oldVex.ID
	newVex.Tooling = oldVex.Tooling
	ver, err := strconv.Atoi(oldVex.Version)
	if err == nil {
		newVex.Version = ver
	}

	// Transcode the statements
	for _, oldStmt := range oldVex.Statements {
		newStmt := Statement{}
		newStmt.Status = Status(oldStmt.Status)
		newStmt.StatusNotes = oldStmt.StatusNotes
		newStmt.ActionStatement = oldStmt.ActionStatement
		newStmt.ActionStatementTimestamp = oldStmt.ActionStatementTimestamp
		newStmt.Justification = Justification(oldStmt.Justification)
		newStmt.ImpactStatement = oldStmt.ImpactStatement
		newStmt.Timestamp = oldStmt.Timestamp

		// Add the vulnerability
		newStmt.Vulnerability = Vulnerability{
			Name:        VulnerabilityID(oldStmt.Vulnerability),
			Description: oldStmt.VulnDescription,
		}

		// Transcode the products from the old statement
		for _, productID := range oldStmt.Products {
			newProduct := Product{
				Component: Component{
					ID: productID,
				},
				Subcomponents: []Subcomponent{},
			}

			for _, sc := range oldStmt.Subcomponents {
				if sc == "" {
					continue
				}
				newProduct.Subcomponents = append(newProduct.Subcomponents, Subcomponent{
					Component: Component{
						ID: sc,
					},
				})
			}
			newStmt.Products = append(newStmt.Products, newProduct)
		}
		newVex.Statements = append(newVex.Statements, newStmt)
	}

	return &newVex, nil
}

type vex001 struct {
	Context    string         `json:"@context"`
	ID         string         `json:"@id"`
	Author     string         `json:"author"`
	AuthorRole string         `json:"role"`
	Timestamp  *time.Time     `json:"timestamp"`
	Version    string         `json:"version"`
	Tooling    string         `json:"tooling,omitempty"`
	Supplier   string         `json:"supplier,omitempty"`
	Statements []statement001 `json:"statements"`
}

type statement001 struct {
	Vulnerability            string     `json:"vulnerability,omitempty"`
	VulnDescription          string     `json:"vuln_description,omitempty"`
	Timestamp                *time.Time `json:"timestamp,omitempty"`
	Products                 []string   `json:"products,omitempty"`
	Subcomponents            []string   `json:"subcomponents,omitempty"`
	Status                   string     `json:"status"`
	StatusNotes              string     `json:"status_notes,omitempty"`
	Justification            string     `json:"justification,omitempty"`
	ImpactStatement          string     `json:"impact_statement,omitempty"`
	ActionStatement          string     `json:"action_statement,omitempty"`
	ActionStatementTimestamp *time.Time `json:"action_statement_timestamp,omitempty"`
}
