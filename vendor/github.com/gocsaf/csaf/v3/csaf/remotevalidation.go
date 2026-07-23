// This file is Free Software under the Apache-2.0 License
// without warranty, see README.md and LICENSES/Apache-2.0.txt for details.
//
// SPDX-License-Identifier: Apache-2.0
//
// SPDX-FileCopyrightText: 2022 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
// Software-Engineering: 2022 Intevation GmbH <https://intevation.de>

package csaf

import (
	"bytes"
	"compress/zlib"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/gocsaf/csaf/v3/internal/misc"
	bolt "go.etcd.io/bbolt"
)

// defaultURL is default URL where to look for
// the validation service.
const (
	defaultURL     = "http://localhost:8082"
	validationPath = "/api/v1/validate"
)

// defaultPresets are the presets to check.
var defaultPresets = []string{"mandatory"}

var (
	validationsBucket = []byte("validations")
	cacheVersionKey   = []byte("version")
	cacheVersion      = []byte("1")
)

// RemoteValidatorOptions are the configuation options
// of the remote validation service.
type RemoteValidatorOptions struct {
	URL     string   `json:"url" toml:"url"`
	Presets []string `json:"presets" toml:"presets"`
	Cache   string   `json:"cache" toml:"cache"`
}

type test struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

// outDocument is the document send to the remote validation service.
type outDocument struct {
	Tests    []test `json:"tests"`
	Document any    `json:"document"`
}

// RemoteTestResult are any given test-result by a remote validator test.
type RemoteTestResult struct {
	Message      string `json:"message"`
	InstancePath string `json:"instancePath"`
}

// RemoteTest is the result of the remote tests
// recieved by the remote validation service.
type RemoteTest struct {
	Name    string             `json:"name"`
	Valid   bool               `json:"isValid"`
	Error   []RemoteTestResult `json:"errors"`
	Warning []RemoteTestResult `json:"warnings"`
	Info    []RemoteTestResult `json:"infos"`
}

// RemoteValidationResult is the document recieved from the remote validation service.
type RemoteValidationResult struct {
	Valid bool         `json:"isValid"`
	Tests []RemoteTest `json:"tests"`
}

type cache interface {
	get(key []byte) ([]byte, error)
	set(key []byte, value []byte) error
	Close() error
}

// RemoteValidator validates an advisory document remotely.
type RemoteValidator interface {
	Validate(doc any) (*RemoteValidationResult, error)
	Close() error
}

// SynchronizedRemoteValidator returns a serialized variant
// of the given remote validator.
func SynchronizedRemoteValidator(validator RemoteValidator) RemoteValidator {
	return &syncedRemoteValidator{RemoteValidator: validator}
}

// remoteValidator is an implementation of an RemoteValidator.
type remoteValidator struct {
	url   string
	tests []test
	cache cache
}

// syncedRemoteValidator is a serialized variant of a remote validator.
type syncedRemoteValidator struct {
	sync.Mutex
	RemoteValidator
}

// Validate implements the validation part of the RemoteValidator interface.
func (srv *syncedRemoteValidator) Validate(doc any) (*RemoteValidationResult, error) {
	srv.Lock()
	defer srv.Unlock()
	return srv.RemoteValidator.Validate(doc)
}

// Validate implements the closing part of the RemoteValidator interface.
func (srv *syncedRemoteValidator) Close() error {
	srv.Lock()
	defer srv.Unlock()
	return srv.RemoteValidator.Close()
}

// prepareTests precompiles the presets for the remote check.
func prepareTests(presets []string) []test {
	if len(presets) == 0 {
		presets = defaultPresets
	}
	tests := make([]test, len(presets))
	for i := range tests {
		tests[i] = test{Type: "preset", Name: presets[i]}
	}
	return tests
}

// prepareURL prepares the URL to be called for validation.
func prepareURL(url string) string {
	if url == "" {
		url = defaultURL
	}
	return url + validationPath
}

// prepareCache sets up the cache if it is configured.
func prepareCache(config string) (cache, error) {
	if config == "" {
		return nil, nil
	}

	db, err := bolt.Open(config, 0600, nil)
	if err != nil {
		return nil, err
	}

	// Create the bucket.
	if err := db.Update(func(tx *bolt.Tx) error {

		// Create a new bucket with version set.
		create := func() error {
			b, err := tx.CreateBucket(validationsBucket)
			if err != nil {
				return err
			}
			return b.Put(cacheVersionKey, cacheVersion)
		}

		b := tx.Bucket(validationsBucket)

		if b == nil { // Bucket does not exists -> create.
			return create()
		}
		// Bucket exists.
		if v := b.Get(cacheVersionKey); !bytes.Equal(v, cacheVersion) {
			// version mismatch -> delete and re-create.
			if err := tx.DeleteBucket(validationsBucket); err != nil {
				return err
			}
			return create()
		}
		return nil
	}); err != nil {
		db.Close()
		return nil, err
	}

	return boltCache{db}, nil
}

// boltCache is cache implementation based on the bolt datastore.
type boltCache struct{ *bolt.DB }

// get implements the fetch part of the cache interface.
func (bc boltCache) get(key []byte) ([]byte, error) {
	var value []byte
	if err := bc.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(validationsBucket)
		value = b.Get(key)
		return nil
	}); err != nil {
		return nil, err
	}
	return value, nil
}

// set implements the store part of the cache interface.
func (bc boltCache) set(key, value []byte) error {
	return bc.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(validationsBucket)
		return b.Put(key, value)
	})
}

// Open opens a new remoteValidator.
func (rvo *RemoteValidatorOptions) Open() (RemoteValidator, error) {
	cache, err := prepareCache(rvo.Cache)
	if err != nil {
		return nil, err
	}
	return &remoteValidator{
		url:   prepareURL(rvo.URL),
		tests: prepareTests(rvo.Presets),
		cache: cache,
	}, nil
}

// Close closes the remote validator.
func (v *remoteValidator) Close() error {
	if v.cache != nil {
		return v.cache.Close()
	}
	return nil
}

// key calculates the key for an advisory document and presets.
func (v *remoteValidator) key(doc any) ([]byte, error) {
	h := sha256.New()
	if err := json.NewEncoder(h).Encode(doc); err != nil {
		return nil, err
	}
	for i := range v.tests {
		if _, err := h.Write([]byte(v.tests[i].Name)); err != nil {
			return nil, err
		}
	}
	return h.Sum(nil), nil
}

// deserialize revives a remote validation result from a cache value.
func deserialize(value []byte) (*RemoteValidationResult, error) {
	r, err := zlib.NewReader(bytes.NewReader(value))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	var rvr RemoteValidationResult
	if err := misc.StrictJSONParse(r, &rvr); err != nil {
		return nil, err
	}
	return &rvr, nil
}

// Validate executes a remote validation of an advisory.
func (v *remoteValidator) Validate(doc any) (*RemoteValidationResult, error) {

	var key []byte

	// First look into cache.
	if v.cache != nil {
		var err error
		if key, err = v.key(doc); err != nil {
			return nil, err
		}
		value, err := v.cache.get(key)
		if err != nil {
			return nil, err
		}
		if value != nil {
			return deserialize(value)
		}
	}

	o := outDocument{
		Document: doc,
		Tests:    v.tests,
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(&o); err != nil {
		return nil, err
	}

	resp, err := http.Post(
		v.url,
		"application/json",
		bytes.NewReader(buf.Bytes()))

	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"POST failed: %s (%d)", resp.Status, resp.StatusCode)
	}

	var (
		zout *zlib.Writer
		rvr  RemoteValidationResult
	)

	if err := func() error {
		defer resp.Body.Close()
		var in io.Reader
		// If we are caching record the incoming data and compress it.
		if key != nil {
			buf.Reset() // reuse the out buffer.
			zout = zlib.NewWriter(&buf)
			in = io.TeeReader(resp.Body, zout)
		} else {
			// no cache -> process directly.
			in = resp.Body
		}
		return misc.StrictJSONParse(in, &rvr)
	}(); err != nil {
		return nil, err
	}

	// Store in cache
	if key != nil {
		if err := zout.Close(); err != nil {
			return nil, err
		}
		// The document is now compressed in the buffer.
		if err := v.cache.set(key, buf.Bytes()); err != nil {
			return nil, err
		}
	}

	return &rvr, nil
}
