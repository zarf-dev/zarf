// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package proxy provides tests for Zarf registry proxy mode.
package proxy

import (
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/zarf-dev/zarf/src/test"
)

var (
	e2e test.ZarfE2ETest
)

func TestMain(m *testing.M) {
	rootDir, err := filepath.Abs("../../../")
	if err != nil {
		log.Fatal(err)
	}
	if err := os.Chdir(rootDir); err != nil {
		log.Fatal(err)
	}

	e2e.ZarfBinPath, err = filepath.Abs(filepath.Join("build", test.GetCLIName()))
	if err != nil {
		log.Fatal(err)
	}

	if _, err := os.Stat(e2e.ZarfBinPath); err != nil {
		log.Fatalf("zarf binary %s not found: %v", e2e.ZarfBinPath, err)
	}
	os.Exit(m.Run())
}
