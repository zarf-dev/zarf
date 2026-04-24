// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	// "context"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/test"
	// "github.com/zarf-dev/zarf/src/pkg/utils/exec"
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
	zarfBinPath, err := filepath.Abs(filepath.Join("build", test.GetCLIName()))
	if err != nil {
		log.Fatal(err)
	}
	e2e.ZarfBinPath = zarfBinPath
	e2e.ApplianceMode = os.Getenv(applianceModeEnvVar) == "true"
	e2e.ApplianceModeKeep = os.Getenv(applianceModeKeepEnvVar) == "true"

	if _, err := os.Stat(e2e.ZarfBinPath); err != nil {
		log.Fatalf("zarf binary %s not found: %v", e2e.ZarfBinPath, err)
	}

	// // If running with-cluster tests validate cluster connectivity early
	// if os.Getenv(skipK8sEnvVar) != "true" && !e2e.ApplianceMode {
	// 	_, _, err := exec.CmdWithContext(context.Background(), exec.Config{}, e2e.ZarfBinPath, "tools", "kubectl", "cluster-info")
	// 	if err != nil {
	// 		log.Fatalf("No cluster found. Ensure a valid kubeconfig is available and cluster is running: %v", err)
	// 	}
	// }
	os.Exit(m.Run())
}
