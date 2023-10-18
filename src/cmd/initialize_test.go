// DEPRECATED_V1.0.0: do not check pkgConfig.InitOpts.RegistryInfo.NodePort, always overwrite it.
package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"testing"

	"github.com/defenseunicorns/zarf/src/config"
)

func TestSetRegistryStorageClass(t *testing.T) {
	// Test case 1: storageClass is set
	pkgConfig.PkgOpts.SetVariables = map[string]string{"REGISTRY_STORAGE_CLASS": "test-storage-class"}
	pkgConfig.InitOpts.StorageClass = ""
	setRegistryStorageClass()
	if pkgConfig.InitOpts.StorageClass != "test-storage-class" {
		t.Errorf("Expected storage class to be set to 'test-storage-class', but got '%s'", pkgConfig.InitOpts.StorageClass)
	}

	// Test case 2: storageClass is not set, use old way
	pkgConfig.PkgOpts.SetVariables = map[string]string{}
	pkgConfig.InitOpts.StorageClass = "old-storage-class"
	setRegistryStorageClass()
	if pkgConfig.PkgOpts.SetVariables["REGISTRY_STORAGE_CLASS"] != "old-storage-class" {
		t.Errorf("Expected storage class to be set to old way value, but got '%s'", pkgConfig.PkgOpts.SetVariables["REGISTRY_STORAGE_CLASS"])
	}

	// Test case 3: neither is set, should be empty
	pkgConfig.PkgOpts.SetVariables = map[string]string{}
	pkgConfig.InitOpts.StorageClass = ""
	setRegistryStorageClass()
	if pkgConfig.PkgOpts.SetVariables["REGISTRY_STORAGE_CLASS"] != "" {
		t.Errorf("Expected storage class to be set to empty string, but got '%s'", pkgConfig.PkgOpts.SetVariables["REGISTRY_STORAGE_CLASS"])
	}
}

// convinence function to setup tests
func setupNodeportTests(r string, n int) {
	pkgConfig.PkgOpts.SetVariables = map[string]string{}
	if r != "" {
		pkgConfig.PkgOpts.SetVariables["REGISTRY_NODEPORT"] = r
	}
	pkgConfig.InitOpts.RegistryInfo.NodePort = n
}

func TestSetRegistryNodePort(t *testing.T) {
	// Test case 1: new way is set, old way is not
	setupNodeportTests("30001", 0)
	setRegistryNodePort()
	if pkgConfig.InitOpts.RegistryInfo.NodePort != 30001 {
		t.Errorf("Expected node port to be set to 30001, but got %d", pkgConfig.InitOpts.RegistryInfo.NodePort)
	}

	// Test case 2: nothing is set, use the default
	setupNodeportTests("", 0)
	setRegistryNodePort()
	if pkgConfig.InitOpts.RegistryInfo.NodePort != config.ZarfInClusterContainerRegistryNodePort {
		t.Errorf("Expected node port to be set to default value, but got %d", pkgConfig.InitOpts.RegistryInfo.NodePort)
	}

	// Test case 3: The old way is set, and the new way is not. We should set the new variable to the old method
	setupNodeportTests("", 30001)
	setRegistryNodePort()
	if c, ok := pkgConfig.PkgOpts.SetVariables["REGISTRY_NODEPORT"]; !ok {
		t.Error("Expected node port to be set to old way value, but it is not set")
	} else {
		if c != "30001" {
			t.Errorf("Expected node port to be set to old way value, but got %s", c)
		}
	}
}

// forked test which will fatal because the old way is invalid
func TestSetRegistryNodePortHelper(t *testing.T) {
	if os.Getenv("TEST_SET_REGISTRY_NODE_PORT_HELPER") != "1" {
		return
	}
	setupNodeportTests("invalid", 0)
	err := func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("Expected test case to fail with fatal error, but it panicked: %v", r)
			}
		}()
		setRegistryNodePort()
		return nil
	}()
	if err == nil {
		os.Exit(0)
	}
	os.Exit(1)

}
func TestSetRegistryNodePortExit(t *testing.T) {
	// Calls the helper
	cmd := exec.Command(os.Args[0], "-test.run=TestSetRegistryNodePortHelper")
	cmd.Env = append(os.Environ(), "TEST_SET_REGISTRY_NODE_PORT_HELPER=1")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	// make sure we got an exitStatus of 1, otherwise the test failed.
	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				if status.ExitStatus() != 1 {
					t.Errorf("Expected test case to fail with exit status 1, but got exit status %d", status.ExitStatus())
				}
			} else {
				t.Errorf("Expected syscall.WaitStatus, but got %T", exitErr.Sys())
			}
		} else {
			t.Error(err)
		}
	}
}
