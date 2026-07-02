// This file is Free Software under the Apache-2.0 License
// without warranty, see README.md and LICENSES/Apache-2.0.txt for details.
//
// SPDX-License-Identifier: Apache-2.0
//
// SPDX-FileCopyrightText: 2021 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
// Software-Engineering: 2021 Intevation GmbH <https://intevation.de>

package csaf

import (
	"encoding/json"
	"io"
	"sort"
	"time"

	"github.com/gocsaf/csaf/v3/internal/misc"
	"github.com/gocsaf/csaf/v3/util"
)

// ROLIEServiceWorkspaceCollectionCategoriesCategory is a category in a ROLIE service collection.
type ROLIEServiceWorkspaceCollectionCategoriesCategory struct {
	Scheme string `json:"scheme"`
	Term   string `json:"term"`
}

// ROLIEServiceWorkspaceCollectionCategories are categories in a ROLIE service collection.
type ROLIEServiceWorkspaceCollectionCategories struct {
	Category []ROLIEServiceWorkspaceCollectionCategoriesCategory `json:"category"`
}

// ROLIEServiceWorkspaceCollection is a collection in a ROLIE service.
type ROLIEServiceWorkspaceCollection struct {
	Title      string                                    `json:"title"`
	HRef       string                                    `json:"href"`
	Categories ROLIEServiceWorkspaceCollectionCategories `json:"categories"`
}

// ROLIEServiceWorkspace is a workspace of a ROLIE service.
type ROLIEServiceWorkspace struct {
	Title      string                            `json:"title"`
	Collection []ROLIEServiceWorkspaceCollection `json:"collection"`
}

// ROLIEService is a ROLIE service.
type ROLIEService struct {
	Workspace []ROLIEServiceWorkspace `json:"workspace"`
}

// ROLIEServiceDocument is a ROLIE service document.
type ROLIEServiceDocument struct {
	Service ROLIEService `json:"service"`
}

// LoadROLIEServiceDocument loads a ROLIE service document from a reader.
func LoadROLIEServiceDocument(r io.Reader) (*ROLIEServiceDocument, error) {
	var rsd ROLIEServiceDocument
	if err := misc.StrictJSONParse(r, &rsd); err != nil {
		return nil, err
	}
	return &rsd, nil
}

// WriteTo saves a ROLIE service document to a writer.
func (rsd *ROLIEServiceDocument) WriteTo(w io.Writer) (int64, error) {
	nw := util.NWriter{Writer: w, N: 0}
	enc := json.NewEncoder(&nw)
	enc.SetIndent("", "  ")
	err := enc.Encode(rsd)
	return nw.N, err
}

// ROLIECategories is a list of ROLIE categories.
type ROLIECategories struct {
	Category []ROLIECategory `json:"category"`
}

// ROLIECategoryDocument is a ROLIE category document.
type ROLIECategoryDocument struct {
	Categories ROLIECategories `json:"categories"`
}

// NewROLIECategoryDocument creates a new ROLIE category document from a list
// of categories.
func NewROLIECategoryDocument(categories ...string) *ROLIECategoryDocument {
	rcd := &ROLIECategoryDocument{}
	rcd.Merge(categories...)
	return rcd
}

// Merge merges the given categories into the existing ones.
// The results indicates if there were changes.
func (rcd *ROLIECategoryDocument) Merge(categories ...string) bool {
	index := util.Set[string]{}
	for i := range rcd.Categories.Category {
		index.Add(rcd.Categories.Category[i].Term)
	}

	oldLen := len(index)

	for _, cat := range categories {
		if index.Contains(cat) {
			continue
		}
		index.Add(cat)
		rcd.Categories.Category = append(
			rcd.Categories.Category, ROLIECategory{Term: cat})
	}

	if len(index) == oldLen {
		// No new categories
		return false
	}

	// Re-establish order.
	sort.Slice(rcd.Categories.Category, func(i, j int) bool {
		return rcd.Categories.Category[i].Term < rcd.Categories.Category[j].Term
	})

	return true
}

// LoadROLIECategoryDocument loads a ROLIE category document from a reader.
func LoadROLIECategoryDocument(r io.Reader) (*ROLIECategoryDocument, error) {
	var rcd ROLIECategoryDocument
	if err := misc.StrictJSONParse(r, &rcd); err != nil {
		return nil, err
	}
	return &rcd, nil
}

// WriteTo saves a ROLIE category document to a writer.
func (rcd *ROLIECategoryDocument) WriteTo(w io.Writer) (int64, error) {
	nw := util.NWriter{Writer: w, N: 0}
	enc := json.NewEncoder(&nw)
	enc.SetIndent("", "  ")
	err := enc.Encode(rcd)
	return nw.N, err
}

// Link for ROLIE.
type Link struct {
	Rel  string `json:"rel"`
	HRef string `json:"href"`
}

// ROLIECategory for ROLIE.
type ROLIECategory struct {
	Scheme string `json:"scheme,omitempty"`
	Term   string `json:"term"`
}

// Summary for ROLIE.
type Summary struct {
	Content string `json:"content"`
}

// Content for ROLIE.
type Content struct {
	Type string `json:"type"`
	Src  string `json:"src"`
}

// Format for ROLIE.
type Format struct {
	Schema  string `json:"schema"`
	Version string `json:"version"`
}

// Entry for ROLIE.
type Entry struct {
	Base        *string          `json:"base,omitempty"`
	LanguageTag *string          `json:"lang,omitempty"`
	Author      *json.RawMessage `json:"author,omitempty"`
	Category    []ROLIECategory  `json:"category,omitempty"`
	Content     Content          `json:"content"`
	Contributor *json.RawMessage `json:"contributor,omitempty"`
	ID          string           `json:"id"`
	Link        []Link           `json:"link"`
	Published   TimeStamp        `json:"published"`
	Rights      *json.RawMessage `json:"rights,omitempty"`
	Source      *json.RawMessage `json:"source,omitempty"`
	Summary     *Summary         `json:"summary,omitempty"`
	Titel       string           `json:"title"`
	Updated     TimeStamp        `json:"updated"`
	Format      Format           `json:"format"`
	Property    *json.RawMessage `json:"property,omitempty"`
}

// FeedData is the content of the ROLIE feed.
type FeedData struct {
	ID       string          `json:"id"`
	Title    string          `json:"title"`
	Link     []Link          `json:"link,omitempty"`
	Category []ROLIECategory `json:"category,omitempty"`
	Updated  TimeStamp       `json:"updated"`
	Entry    []*Entry        `json:"entry"`
}

// ROLIEFeed is a ROLIE feed.
type ROLIEFeed struct {
	Feed FeedData `json:"feed"`
}

// LoadROLIEFeed loads a ROLIE feed from a reader.
func LoadROLIEFeed(r io.Reader) (*ROLIEFeed, error) {
	var rf ROLIEFeed
	if err := misc.StrictJSONParse(r, &rf); err != nil {
		return nil, err
	}
	return &rf, nil
}

// WriteTo saves a ROLIE feed to a writer.
func (rf *ROLIEFeed) WriteTo(w io.Writer) (int64, error) {
	nw := util.NWriter{Writer: w, N: 0}
	enc := json.NewEncoder(&nw)
	enc.SetIndent("", "  ")
	err := enc.Encode(rf)
	return nw.N, err
}

// EntryByID looks up an entry by its ID.
// Returns nil if no such entry was found.
func (rf *ROLIEFeed) EntryByID(id string) *Entry {
	for _, entry := range rf.Feed.Entry {
		if entry.ID == id {
			return entry
		}
	}
	return nil
}

// Entries visits the entries of this feed.
func (rf *ROLIEFeed) Entries(fn func(*Entry)) {
	for _, e := range rf.Feed.Entry {
		fn(e)
	}
}

// SortEntriesByUpdated sorts all the entries in the feed
// by their update times.
func (rf *ROLIEFeed) SortEntriesByUpdated() {
	entries := rf.Feed.Entry
	sort.Slice(entries, func(i, j int) bool {
		return time.Time(entries[j].Updated).Before(time.Time(entries[i].Updated))
	})
}

// CountEntries returns the number of entries within the feed
func (rf *ROLIEFeed) CountEntries() int {
	return len(rf.Feed.Entry)
}
