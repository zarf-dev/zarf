// This file is Free Software under the Apache-2.0 License
// without warranty, see README.md and LICENSES/Apache-2.0.txt for details.
//
// SPDX-License-Identifier: Apache-2.0
//
// SPDX-FileCopyrightText: 2022 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
// Software-Engineering: 2022 Intevation GmbH <https://intevation.de>

package csaf

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gocsaf/csaf/v3/internal/misc"
	"github.com/gocsaf/csaf/v3/util"
)

// AdvisoryFile constructs the urls of a remote file.
type AdvisoryFile interface {
	slog.LogValuer
	URL() string
	SHA256URL() string
	SHA512URL() string
	SignURL() string
	IsDirectory() bool
}

// PlainAdvisoryFile contains all relevant urls of a remote file.
type PlainAdvisoryFile struct {
	Path   string
	SHA256 string
	SHA512 string
	Sign   string
}

// URL returns the URL of this advisory.
func (paf PlainAdvisoryFile) URL() string { return paf.Path }

// SHA256URL returns the URL of SHA256 hash file of this advisory.
func (paf PlainAdvisoryFile) SHA256URL() string { return paf.SHA256 }

// SHA512URL returns the URL of SHA512 hash file of this advisory.
func (paf PlainAdvisoryFile) SHA512URL() string { return paf.SHA512 }

// SignURL returns the URL of signature file of this advisory.
func (paf PlainAdvisoryFile) SignURL() string { return paf.Sign }

// IsDirectory returns true, if was fetched via directory feeds.
func (paf PlainAdvisoryFile) IsDirectory() bool { return false }

// LogValue implements [slog.LogValuer]
func (paf PlainAdvisoryFile) LogValue() slog.Value {
	return slog.GroupValue(slog.String("url", paf.URL()))
}

// DirectoryAdvisoryFile only contains the base file path.
// The hash and signature files are directly constructed by extending
// the file name.
type DirectoryAdvisoryFile struct {
	Path string
}

// URL returns the URL of this advisory.
func (daf DirectoryAdvisoryFile) URL() string { return daf.Path }

// SHA256URL returns the URL of SHA256 hash file of this advisory.
func (daf DirectoryAdvisoryFile) SHA256URL() string { return daf.Path + ".sha256" }

// SHA512URL returns the URL of SHA512 hash file of this advisory.
func (daf DirectoryAdvisoryFile) SHA512URL() string { return daf.Path + ".sha512" }

// SignURL returns the URL of signature file of this advisory.
func (daf DirectoryAdvisoryFile) SignURL() string { return daf.Path + ".asc" }

// IsDirectory returns true, if was fetched via directory feeds.
func (daf DirectoryAdvisoryFile) IsDirectory() bool { return true }

// LogValue implements [slog.LogValuer]
func (daf DirectoryAdvisoryFile) LogValue() slog.Value {
	return slog.GroupValue(slog.String("url", daf.URL()))
}

// AdvisoryFileProcessor implements the extraction of
// advisory file names from a given provider metadata.
type AdvisoryFileProcessor struct {
	AgeAccept func(time.Time) bool
	Log       func(loglevel slog.Level, format string, args ...any)
	client    util.Client
	expr      *util.PathEval
	doc       any
	pmdURL    *url.URL
}

// NewAdvisoryFileProcessor constructs a filename extractor
// for a given metadata document.
func NewAdvisoryFileProcessor(
	client util.Client,
	expr *util.PathEval,
	doc any,
	pmdURL *url.URL,
) *AdvisoryFileProcessor {
	return &AdvisoryFileProcessor{
		client: client,
		expr:   expr,
		doc:    doc,
		pmdURL: pmdURL,
	}
}

// empty checks if list of strings contains at least one none empty string.
func empty(arr []string) bool {
	for _, s := range arr {
		if s != "" {
			return false
		}
	}
	return true
}

// Process extracts the advisory filenames and passes them with
// the corresponding label to fn.
func (afp *AdvisoryFileProcessor) Process(
	fn func(TLPLabel, []AdvisoryFile) error,
) error {
	lg := afp.Log
	if lg == nil {
		lg = func(loglevel slog.Level, format string, args ...any) {
			slog.Log(context.Background(), loglevel, "AdvisoryFileProcessor.Process: "+format, args...)
		}
	}

	// Check if we have ROLIE feeds.
	rolie, err := afp.expr.Eval(
		"$.distributions[*].rolie.feeds", afp.doc)
	if err != nil {
		lg(slog.LevelError, "rolie check failed", "err", err)
		return err
	}

	fs, hasRolie := rolie.([]any)
	hasRolie = hasRolie && len(fs) > 0

	if hasRolie {
		var feeds [][]Feed
		if err := util.ReMarshalJSON(&feeds, rolie); err != nil {
			return err
		}
		lg(slog.LevelInfo, "Found ROLIE feed(s)", "length", len(feeds))

		for _, feed := range feeds {
			if err := afp.processROLIE(feed, fn); err != nil {
				return err
			}
		}
	} else {
		// No rolie feeds -> try to load files from index.txt

		directoryURLs, err := afp.expr.Eval(
			"$.distributions[*].directory_url", afp.doc)

		var dirURLs []string

		if err != nil {
			lg(slog.LevelError, "extracting directory URLs failed", "err", err)
		} else {
			var ok bool
			dirURLs, ok = util.AsStrings(directoryURLs)
			if !ok {
				lg(slog.LevelError, "directory_urls are not strings")
			}
		}

		// Not found -> fall back to PMD url
		if empty(dirURLs) {
			baseURL, err := util.BaseURL(afp.pmdURL)
			if err != nil {
				return err
			}
			dirURLs = []string{baseURL}
		}

		for _, base := range dirURLs {
			if base == "" {
				continue
			}

			// Use changes.csv to be able to filter by age.
			files, err := afp.loadChanges(base, lg)
			if err != nil {
				return err
			}
			// XXX: Is treating as white okay? better look into the advisories?
			if err := fn(TLPLabelWhite, files); err != nil {
				return err
			}
		}
	} // TODO: else scan directories?
	return nil
}

// loadChanges loads baseURL/changes.csv and returns a list of files
// prefixed by baseURL/.
func (afp *AdvisoryFileProcessor) loadChanges(
	baseURL string,
	lg func(slog.Level, string, ...any),
) ([]AdvisoryFile, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	changesURL := base.JoinPath("changes.csv").String()

	resp, err := afp.client.Get(changesURL)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching %s failed. Status code %d (%s)",
			changesURL, resp.StatusCode, resp.Status)
	}

	defer resp.Body.Close()
	var files []AdvisoryFile
	c := csv.NewReader(resp.Body)
	const (
		pathColumn = 0
		timeColumn = 1
	)
	for line := 1; ; line++ {
		r, err := c.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(r) < 2 {
			lg(slog.LevelError, "Not enough columns", "line", line)
			continue
		}
		t, err := time.Parse(time.RFC3339, r[timeColumn])
		if err != nil {
			lg(slog.LevelError, "Invalid time stamp in line", "url", changesURL, "line", line, "err", err)
			continue
		}
		// Apply date range filtering.
		if afp.AgeAccept != nil && !afp.AgeAccept(t) {
			continue
		}
		path := r[pathColumn]
		if _, err := url.Parse(path); err != nil {
			lg(slog.LevelError, "Contains an invalid URL", "url", changesURL, "path", path, "line", line)
			continue
		}

		pathURL, err := url.Parse(path)
		if err != nil {
			return nil, err
		}

		files = append(files,
			DirectoryAdvisoryFile{Path: misc.JoinURL(base, pathURL).String()})
	}
	return files, nil
}

func (afp *AdvisoryFileProcessor) processROLIE(
	labeledFeeds []Feed,
	fn func(TLPLabel, []AdvisoryFile) error,
) error {
	for i := range labeledFeeds {
		feed := &labeledFeeds[i]
		if feed.URL == nil {
			continue
		}
		feedURL, err := url.Parse(string(*feed.URL))
		if err != nil {
			slog.Error("Invalid URL in feed", "feed", *feed.URL, "err", err)
			continue
		}
		slog.Info("Got feed URL", "feed", feedURL)

		fb, err := util.BaseURL(feedURL)
		if err != nil {
			slog.Error("Invalid feed base URL", "url", fb, "err", err)
			continue
		}

		res, err := afp.client.Get(feedURL.String())
		if err != nil {
			slog.Error("Cannot get feed", "err", err)
			continue
		}
		if res.StatusCode != http.StatusOK {
			slog.Error("Fetching failed",
				"url", feedURL, "status_code", res.StatusCode, "status", res.Status)
			continue
		}
		rfeed, err := func() (*ROLIEFeed, error) {
			defer res.Body.Close()
			return LoadROLIEFeed(res.Body)
		}()
		if err != nil {
			slog.Error("Loading ROLIE feed failed", "err", err)
			continue
		}

		var files []AdvisoryFile

		resolve := func(u string) string {
			if u == "" {
				return ""
			}
			p, err := url.Parse(u)
			if err != nil {
				slog.Error("Invalid URL", "url", u, "err", err)
				return ""
			}
			return p.String()
		}

		rfeed.Entries(func(entry *Entry) {
			// Filter if we have date checking.
			if afp.AgeAccept != nil {
				if t := time.Time(entry.Updated); !t.IsZero() && !afp.AgeAccept(t) {
					return
				}
			}

			var self, sha256, sha512, sign string

			for i := range entry.Link {
				link := &entry.Link[i]
				lower := strings.ToLower(link.HRef)
				switch link.Rel {
				case "self":
					self = resolve(link.HRef)
				case "signature":
					sign = resolve(link.HRef)
				case "hash":
					switch {
					case strings.HasSuffix(lower, ".sha256"):
						sha256 = resolve(link.HRef)
					case strings.HasSuffix(lower, ".sha512"):
						sha512 = resolve(link.HRef)
					}
				}
			}

			if self == "" {
				return
			}

			var file AdvisoryFile

			switch {
			case sha256 == "" && sha512 == "":
				slog.Error("No hash listed on ROLIE feed", "file", self)
				return
			case sign == "":
				slog.Error("No signature listed on ROLIE feed", "file", self)
				return
			default:
				file = PlainAdvisoryFile{self, sha256, sha512, sign}
			}

			files = append(files, file)
		})

		var label TLPLabel
		if feed.TLPLabel != nil {
			label = *feed.TLPLabel
		} else {
			label = "unknown"
		}

		if err := fn(label, files); err != nil {
			return err
		}
	}
	return nil
}
