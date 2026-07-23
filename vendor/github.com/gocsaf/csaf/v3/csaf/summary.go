// This file is Free Software under the Apache-2.0 License
// without warranty, see README.md and LICENSES/Apache-2.0.txt for details.
//
// SPDX-License-Identifier: Apache-2.0
//
// SPDX-FileCopyrightText: 2021 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
// Software-Engineering: 2021 Intevation GmbH <https://intevation.de>

package csaf

import (
	"time"

	"github.com/gocsaf/csaf/v3/util"
)

const (
	idExpr                 = `$.document.tracking.id`
	titleExpr              = `$.document.title`
	publisherExpr          = `$.document.publisher`
	initialReleaseDateExpr = `$.document.tracking.initial_release_date`
	currentReleaseDateExpr = `$.document.tracking.current_release_date`
	tlpLabelExpr           = `$.document.distribution.tlp.label`
	summaryExpr            = `$.document.notes[? @.category=="summary" || @.type=="summary"].text`
	statusExpr             = `$.document.tracking.status`
)

// AdvisorySummary is a summary of some essentials of an CSAF advisory.
type AdvisorySummary struct {
	ID                 string
	Title              string
	Publisher          *Publisher
	InitialReleaseDate time.Time
	CurrentReleaseDate time.Time
	Summary            string
	TLPLabel           string
	Status             string
}

// NewAdvisorySummary creates a summary from an advisory doc
// with the help of an expression evaluator expr.
func NewAdvisorySummary(
	pe *util.PathEval,
	doc any,
) (*AdvisorySummary, error) {

	e := &AdvisorySummary{
		Publisher: new(Publisher),
	}

	if err := pe.Match([]util.PathEvalMatcher{
		{Expr: idExpr, Action: util.StringMatcher(&e.ID)},
		{Expr: titleExpr, Action: util.StringMatcher(&e.Title)},
		{Expr: currentReleaseDateExpr, Action: util.TimeMatcher(&e.CurrentReleaseDate, time.RFC3339)},
		{Expr: initialReleaseDateExpr, Action: util.TimeMatcher(&e.InitialReleaseDate, time.RFC3339)},
		{Expr: summaryExpr, Action: util.StringMatcher(&e.Summary), Optional: true},
		{Expr: tlpLabelExpr, Action: util.StringMatcher(&e.TLPLabel), Optional: true},
		{Expr: publisherExpr, Action: util.ReMarshalMatcher(e.Publisher)},
		{Expr: statusExpr, Action: util.StringMatcher(&e.Status)},
	}, doc); err != nil {
		return nil, err
	}

	return e, nil
}
