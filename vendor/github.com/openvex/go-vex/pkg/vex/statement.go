// Copyright 2023 The OpenVEX Authors
// SPDX-License-Identifier: Apache-2.0

package vex

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// A Statement is a declaration conveying a single [status] for a single
// [vul_id] for one or more [product_id]s. A VEX Statement exists within a VEX
// Document.
type Statement struct {
	// ID is an optional identifier for the statement. It takes an IRI and must
	// be unique for each statement in the document.
	ID string `json:"@id,omitempty"`

	// [vul_id] SHOULD use existing and well known identifiers, for example:
	// CVE, the Global Security Database (GSD), or a supplier’s vulnerability
	// tracking system. It is expected that vulnerability identification systems
	// are external to and maintained separately from VEX.
	//
	// [vul_id] MAY be URIs or URLs.
	// [vul_id] MAY be arbitrary and MAY be created by the VEX statement [author].
	Vulnerability Vulnerability `json:"vulnerability,omitempty"`

	// Timestamp is the time at which the information expressed in the Statement
	// was known to be true.
	Timestamp *time.Time `json:"timestamp,omitempty"`

	// LastUpdated records the time when the statement last had a modification
	LastUpdated *time.Time `json:"last_updated,omitempty"`

	// Product
	// Product details MUST specify what Status applies to.
	// Product details MUST include [product_id] and MAY include [subcomponent_id].
	Products []Product `json:"products,omitempty"`

	// A VEX statement MUST provide Status of the vulnerabilities with respect to the
	// products and components listed in the statement. Status MUST be one of the
	// Status const values, some of which have further options and requirements.
	Status Status `json:"status"`

	// [status_notes] MAY convey information about how [status] was determined
	// and MAY reference other VEX information.
	StatusNotes string `json:"status_notes,omitempty"`

	// For ”not_affected” status, a VEX statement MUST include a status Justification
	// that further explains the status.
	Justification Justification `json:"justification,omitempty"`

	// For ”not_affected” status, a VEX statement MAY include an ImpactStatement
	// that contains a description why the vulnerability cannot be exploited.
	ImpactStatement string `json:"impact_statement,omitempty"`

	// For "affected" status, a VEX statement MUST include an ActionStatement that
	// SHOULD describe actions to remediate or mitigate [vul_id].
	ActionStatement          string     `json:"action_statement,omitempty"`
	ActionStatementTimestamp *time.Time `json:"action_statement_timestamp,omitempty"`
}

// Validate checks to see whether the given Statement is valid. If it's not, an
// error is returned explaining the reason the Statement is invalid. Otherwise,
// nil is returned.
func (stmt *Statement) Validate() error {
	if s := stmt.Status; !s.Valid() {
		return fmt.Errorf("invalid status value %q, must be one of [%s]", s, strings.Join(Statuses(), ", "))
	}

	switch s := stmt.Status; s {
	case StatusNotAffected:
		// require a justification
		j := stmt.Justification
		is := stmt.ImpactStatement
		if j == "" && is == "" {
			return fmt.Errorf("either justification or impact statement must be defined when using status %q", s)
		}

		if j != "" && !j.Valid() {
			return fmt.Errorf("invalid justification value %q, must be one of [%s]", j, strings.Join(Justifications(), ", "))
		}

		// irrelevant fields should not be set
		if v := stmt.ActionStatement; v != "" {
			return fmt.Errorf("action statement should not be set when using status %q (was set to %q)", s, v)
		}

	case StatusAffected:
		// irrelevant fields should not be set
		if v := stmt.Justification; v != "" {
			return fmt.Errorf("justification should not be set when using status %q (was set to %q)", s, v)
		}

		if v := stmt.ImpactStatement; v != "" {
			return fmt.Errorf("impact statement should not be set when using status %q (was set to %q)", s, v)
		}

		// action statement is now required
		if v := stmt.ActionStatement; v == "" {
			return fmt.Errorf("action statement must be set when using status %q", s)
		}

	case StatusUnderInvestigation:
		// irrelevant fields should not be set
		if v := stmt.Justification; v != "" {
			return fmt.Errorf("justification should not be set when using status %q (was set to %q)", s, v)
		}

		if v := stmt.ImpactStatement; v != "" {
			return fmt.Errorf("impact statement should not be set when using status %q (was set to %q)", s, v)
		}

		if v := stmt.ActionStatement; v != "" {
			return fmt.Errorf("action statement should not be set when using status %q (was set to %q)", s, v)
		}

	case StatusFixed:
		// irrelevant fields should not be set
		if v := stmt.Justification; v != "" {
			return fmt.Errorf("justification should not be set when using status %q (was set to %q)", s, v)
		}

		if v := stmt.ImpactStatement; v != "" {
			return fmt.Errorf("impact statement should not be set when using status %q (was set to %q)", s, v)
		}

		if v := stmt.ActionStatement; v != "" {
			return fmt.Errorf("action statement should not be set when using status %q (was set to %q)", s, v)
		}
	}

	return nil
}

// SortStatements does an "in-place" sort of the given slice of VEX statements.
//
// The documentTimestamp parameter is needed because statements without timestamps inherit the timestamp of the document.
func SortStatements(stmts []Statement, documentTimestamp time.Time) {
	sort.SliceStable(stmts, func(i, j int) bool {
		// TODO: Add methods for aliases
		vulnComparison := strings.Compare(string(stmts[i].Vulnerability.Name), string(stmts[j].Vulnerability.Name))
		if vulnComparison != 0 {
			// i.e. different vulnerabilities; sort by string comparison
			return vulnComparison < 0
		}

		// i.e. the same vulnerability; sort statements by timestamp

		iTime := stmts[i].Timestamp
		if iTime == nil || iTime.IsZero() {
			iTime = &documentTimestamp
		}

		jTime := stmts[j].Timestamp
		if jTime == nil || jTime.IsZero() {
			jTime = &documentTimestamp
		}

		if iTime == nil {
			return false
		}

		if jTime == nil {
			return true
		}

		return iTime.Before(*jTime)
	})
}

// Matches returns true if the statement matches the specified vulnerability
// identifier, the VEX product and any of the identifiers from the received list.
func (stmt *Statement) Matches(vuln, product string, subcomponents []string) bool {
	if !stmt.Vulnerability.Matches(vuln) {
		return false
	}

	for i := range stmt.Products {
		if len(subcomponents) == 0 {
			if stmt.Products[i].Matches(product, "") {
				return true
			}
		}

		for _, sc := range subcomponents {
			if stmt.Products[i].Matches(product, sc) {
				return true
			}
		}
	}
	return false
}

// MatchesProduct returns true if the statement matches the identifier string
// with an optional subcomponent identifier
func (stmt *Statement) MatchesProduct(identifier, subidentifier string) bool {
	for _, p := range stmt.Products {
		if p.Matches(identifier, subidentifier) {
			return true
		}
	}
	return false
}

// MarshalJSON the document object overrides its marshaling function to normalize
// the timezones in all dates to Zulu.
func (stmt *Statement) MarshalJSON() ([]byte, error) {
	type alias Statement
	var ts, lu string

	if stmt.Timestamp != nil {
		ts = stmt.Timestamp.UTC().Format(time.RFC3339Nano)
	}
	if stmt.LastUpdated != nil {
		lu = stmt.LastUpdated.UTC().Format(time.RFC3339Nano)
	}

	return json.Marshal(&struct {
		*alias
		TimeZonedTimestamp   string `json:"timestamp,omitempty"`
		TimeZonedLastUpdated string `json:"last_updated,omitempty"`
	}{
		alias:                (*alias)(stmt),
		TimeZonedTimestamp:   ts,
		TimeZonedLastUpdated: lu,
	})
}

// DeepCopyInto copies the receiver and writes its value into out.
func (stmt *Statement) DeepCopyInto(out *Statement) {
	*out = *stmt

	if stmt.Timestamp != nil {
		*out = *stmt
		out.Timestamp = new(time.Time)
		*out.Timestamp = *stmt.Timestamp
	}

	if stmt.LastUpdated != nil {
		*out = *stmt
		out.LastUpdated = new(time.Time)
		*out.LastUpdated = *stmt.LastUpdated
	}

	if stmt.Products != nil {
		*out = *stmt
		out.Products = make([]Product, len(stmt.Products))
		copy(out.Products, stmt.Products)
	}

	*out = *stmt
	out.Vulnerability = Vulnerability{}
	stmt.Vulnerability.DeepCopyInto(&out.Vulnerability)

	if stmt.Justification != "" {
		*out = *stmt
		out.Justification = stmt.Justification
	}

	if stmt.ActionStatementTimestamp != nil {
		*out = *stmt
		out.ActionStatementTimestamp = new(time.Time)
		*out.ActionStatementTimestamp = *stmt.ActionStatementTimestamp
	}
}

// DeepCopy copies the receiver and returns a new Statement.
func (stmt *Statement) DeepCopy() *Statement {
	if stmt == nil {
		return nil
	}
	out := new(Statement)
	stmt.DeepCopyInto(out)
	return out
}
