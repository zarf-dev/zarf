// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/test"
)

var (
	e2e test.ZarfE2ETest //nolint:gochecknoglobals
)

const (
	applianceModeEnvVar     = "APPLIANCE_MODE"
	applianceModeKeepEnvVar = "APPLIANCE_MODE_KEEP"
	skipK8sEnvVar           = "SKIP_K8S"
)

func TestMain(m *testing.M) {
	rootDir, err := filepath.Abs("../../../")
	if err != nil {
		log.Fatal(err)
	}
	if err := os.Chdir(rootDir); err != nil {
		log.Fatal(err)
	}

	e2e.Arch = config.GetArch()
	e2e.ZarfBinPath = filepath.Join("build", test.GetCLIName())
	e2e.ApplianceMode = os.Getenv(applianceModeEnvVar) == "true"
	e2e.ApplianceModeKeep = os.Getenv(applianceModeKeepEnvVar) == "true"

	message.SetLogLevel(message.TraceLevel)

	if _, err := os.Stat(e2e.ZarfBinPath); err != nil {
		log.Fatalf("zarf binary %s not found: %v", e2e.ZarfBinPath, err)
	}
	os.Exit(m.Run())
}
