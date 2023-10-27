// DEPRECATED_V1.0.0: do not check pkgConfig.InitOpts.RegistryInfo.NodePort, always overwrite it.
package cmd

import (
	"testing"
)

func TestSetRegistryStorageClass(t *testing.T) {
	// Test case 1: storageClass is set
	pkgConfig.PkgOpts.SetVariables = map[string]string{"REGISTRY_STORAGE_CLASS": "test-storage-class"}
	storageClassArg = ""
	setRegistryStorageClass()
	if storageClassArg != "test-storage-class" {
		t.Errorf("Expected storage class to be set to 'test-storage-class', but got '%s'", storageClassArg)
	}

	// Test case 2: storageClass is not set, use old way
	pkgConfig.PkgOpts.SetVariables = map[string]string{}
	localStorageClass = "old-storage-class"
	setRegistryStorageClass()
	if pkgConfig.PkgOpts.SetVariables["REGISTRY_STORAGE_CLASS"] != "old-storage-class" {
		t.Errorf("Expected storage class to be set to old way value, but got '%s'", pkgConfig.PkgOpts.SetVariables["REGISTRY_STORAGE_CLASS"])
	}

	// Test case 3: neither is set, should be empty
	pkgConfig.PkgOpts.SetVariables = map[string]string{}
	localStorageClass = ""
	setRegistryStorageClass()
	if pkgConfig.PkgOpts.SetVariables["REGISTRY_STORAGE_CLASS"] != "" {
		t.Errorf("Expected storage class to be set to empty string, but got '%s'", pkgConfig.PkgOpts.SetVariables["REGISTRY_STORAGE_CLASS"])
	}

}
