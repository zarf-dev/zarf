// Copyright 2023 The OpenVEX Authors
// SPDX-License-Identifier: Apache-2.0

package vex

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"
	"strings"
)

type MergeOptions struct {
	DocumentID      string   // ID to use in the new document
	Author          string   // Author to use in the new document
	AuthorRole      string   // Role of the document author
	Products        []string // Product IDs to consider
	Vulnerabilities []string // IDs of vulnerabilities to merge
}

// MergeDocuments is a convenience wrapper over MergeDocumentsWithOptions
// that does not take options.
func MergeDocuments(docs []*VEX) (*VEX, error) {
	return MergeDocumentsWithOptions(&MergeOptions{}, docs)
}

// Merge combines the statements from a number of documents into
// a new one, preserving time context from each of them.
func MergeDocumentsWithOptions(mergeOpts *MergeOptions, docs []*VEX) (*VEX, error) {
	if len(docs) == 0 {
		return nil, fmt.Errorf("at least one vex document is required to merge")
	}

	docID := mergeOpts.DocumentID
	// If no document id is specified we compute a
	// deterministic ID using the merged docs
	if docID == "" {
		ids := []string{}
		for i, d := range docs {
			if d.ID == "" {
				ids = append(ids, fmt.Sprintf("VEX-DOC-%d", i))
			} else {
				ids = append(ids, d.ID)
			}
		}

		sort.Strings(ids)
		h := sha256.New()
		h.Write([]byte(strings.Join(ids, ":")))
		// Hash the sorted IDs list
		docID = fmt.Sprintf("merged-vex-%x", h.Sum(nil))
	}

	newDoc := New()

	newDoc.ID = docID
	if author := mergeOpts.Author; author != "" {
		newDoc.Author = author
	}
	if authorRole := mergeOpts.AuthorRole; authorRole != "" {
		newDoc.AuthorRole = authorRole
	}

	ss := []Statement{}

	// Create an inverse dict of products and vulnerabilities to filter
	// these will only be used if ids to filter on are defined in the options.
	iProds := map[string]struct{}{}
	iVulns := map[string]struct{}{}
	for _, id := range mergeOpts.Products {
		iProds[id] = struct{}{}
	}
	for _, id := range mergeOpts.Vulnerabilities {
		iVulns[id] = struct{}{}
	}

	for _, doc := range docs {
		for _, s := range doc.Statements { //nolint:gocritic // this IS supposed to copy
			matchesProduct := false
			for id := range iProds {
				if s.MatchesProduct(id, "") {
					matchesProduct = true
					break
				}
			}
			if len(iProds) > 0 && !matchesProduct {
				continue
			}

			matchesVuln := false
			for id := range iVulns {
				if s.Vulnerability.Matches(id) {
					matchesVuln = true
					break
				}
			}
			if len(iVulns) > 0 && !matchesVuln {
				continue
			}

			// If statement does not have a timestamp, cascade
			// the timestamp down from the document.
			// See https://github.com/chainguard-dev/vex/issues/49
			if s.Timestamp == nil {
				if doc.Timestamp == nil {
					return nil, errors.New("unable to cascade timestamp from doc to timeless statement")
				}
				s.Timestamp = doc.Timestamp
			}

			ss = append(ss, s)
		}
	}

	SortStatements(ss, *newDoc.Timestamp)

	newDoc.Statements = ss

	return &newDoc, nil
}

// SortDocuments sorts and returns a slice of documents based on their date.
// VEXes should be applied sequentially in chronological order as they capture
// knowledge about an artifact as it changes over time.
func SortDocuments(docs []*VEX) []*VEX {
	sort.Slice(docs, func(i, j int) bool {
		if docs[j].Timestamp == nil {
			return true
		}
		if docs[i].Timestamp == nil {
			return false
		}
		return docs[i].Timestamp.Before(*(docs[j].Timestamp))
	})
	return docs
}
