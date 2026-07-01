// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/state"
)

func TestComponentStatus(t *testing.T) {
	t.Log("E2E: Component Status")
	tmpDir := t.TempDir()
	_, _, err := e2e.Zarf(t, "package", "create", "src/test/packages/37-component-status", "-o", tmpDir, "--confirm")
	require.NoError(t, err)
	packageName := fmt.Sprintf("zarf-package-component-status-%s.tar.zst", e2e.Arch)
	path := filepath.Join(tmpDir, packageName)
	// Stop channel getting the zarf state
	stop := make(chan bool)
	// Error channel to return any errors from the goroutine. Testify doesn't like require in a goroutine
	errCh := make(chan error, 1)
	var wg sync.WaitGroup
	wg.Add(1)
	// Goroutine to wait for the package to show "deploying" status
	go func() {
		defer wg.Done()
		// Give extra time to build and push the package
		ticker := time.NewTicker(30 * time.Second)
		for {
			select {
			case <-ticker.C:
				t.Log("Timed out waiting for package to deploy")
				errCh <- fmt.Errorf("timed out waiting for package to deploy")
				return
			case <-stop:
				return
			default:
				stdOut, _, err := e2e.Kubectl(t, "get", "secret", "zarf-package-component-status", "-n", "zarf", "-o", "jsonpath={.data.data}")
				if err != nil {
					// Wait for the secret to be created and try again
					time.Sleep(2 * time.Second)
					continue
				}
				deployedPackage, err := getDeployedPackage(stdOut)
				if err != nil {
					errCh <- err
					return
				}
				if len(deployedPackage.DeployedComponents) != 1 {
					errCh <- fmt.Errorf("expected 1 component got %d", len(deployedPackage.DeployedComponents))
					return
				}
				compStatus := deployedPackage.DeployedComponents[0].Status
				if compStatus != state.ComponentStatusDeploying {
					errCh <- fmt.Errorf("expected component status %s got %s", state.ComponentStatusDeploying, compStatus)
					return
				}
				if deployedPackage.Status != state.PackageStatusDeploying {
					errCh <- fmt.Errorf("expected package status %s got %s", state.PackageStatusDeploying, deployedPackage.Status)
					return
				}
				time.Sleep(2 * time.Second)
				return
			}
		}
	}()
	stdOut, stdErr, err := e2e.Zarf(t, "package", "deploy", path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	close(stop)
	wg.Wait()
	select {
	case err := <-errCh:
		require.NoError(t, err)
	default:
	}
	// Verify that the component and package statuses are "succeeded" and source is recorded
	stdOut, stdErr, err = e2e.Kubectl(t, "get", "secret", "zarf-package-component-status", "-n", "zarf", "-o", "jsonpath={.data.data}")
	require.NoError(t, err, stdOut, stdErr)
	deployedPackage, err := getDeployedPackage(stdOut)
	require.NoError(t, err)
	require.Len(t, deployedPackage.DeployedComponents, 1)
	require.Equal(t, state.ComponentStatusSucceeded, deployedPackage.DeployedComponents[0].Status)
	require.Equal(t, state.PackageStatusSucceeded, deployedPackage.Status)
	require.Equal(t, path, deployedPackage.Source)
	// Remove the package
	t.Cleanup(func() {
		stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", "component-status", "--confirm")
		require.NoError(t, err, stdOut, stdErr)
	})
}

func getDeployedPackage(secret string) (*state.DeployedPackage, error) {
	deployedPackage := &state.DeployedPackage{}
	decoded, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(decoded, &deployedPackage)
	if err != nil {
		return nil, err
	}
	return deployedPackage, nil
}
