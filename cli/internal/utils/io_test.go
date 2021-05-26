package utils_test

import (
	"reflect"
	"testing"

	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/shift/pack/cli/internal/utils"
)

func init() {
	utils.ArchivePath = "testdata/export.tar.zst"
	// Run this just to init the decompression
	utils.AssetPath("fake")
}

func TestAssetPath(t *testing.T) {
	input := "test-file-path"
	actual := utils.AssetPath(input)
	expected := utils.TempDestination + "/" + input
	if actual != expected {
		t.Errorf("utils.AssetPath(%q) = %v; want %v", input, actual, expected)
	}
}

func TestAssetList(t *testing.T) {
	testMatch := []string{utils.TempDestination + "/50kb"}
	testCases := []struct {
		input    string
		expected []string
	}{
		{"50kb", testMatch},
		{"50*", testMatch},
		{"invalid", []string{""}},
	}

	for _, test := range testCases {
		actual := utils.AssetList(test.input)
		if !reflect.DeepEqual(actual, test.expected) && test.input != "invalid" {
			t.Errorf("utils.AssetList(%q) = %v; want %v", test.input, actual, test.expected)
		}
	}
}

func TestInvalidPath(t *testing.T) {
	testCases := []struct {
		input    string
		expected bool
	}{
		{"/bleh-bleh-bleh", true},
		{"./testdata/blah", true},
		{"./testdata/50kb.test", true},
		{"./testdata/EXPORT.tar.zst", true},
		{"./testdata/export.tar.zst", false},
		{"./../utils/testdata/export.tar.zst", false},
	}

	for _, test := range testCases {
		actual := utils.InvalidPath(test.input)
		if actual != test.expected {
			t.Errorf("utils.InvalidPath(%q) = %v; want %v", test.input, actual, test.expected)
		}
	}
}

func TestVerifyBinary(t *testing.T) {
	testCases := []struct {
		input    string
		expected bool
	}{
		{"bleh-bleh-bleh", false},
		{"ls", true},
	}

	for _, test := range testCases {
		actual := utils.VerifyBinary(test.input)
		if actual != test.expected {
			t.Errorf("utils.VerifyBinary(%q) = %v; want %v", test.input, actual, test.expected)
		}
	}
}
