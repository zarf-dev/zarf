package utils_test

import (
	"testing"

	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/shift/cli/src/internal/utils"
)

type TestCase struct {
	input    string
	expected string
}

func TestGetSha256(t *testing.T) {
	testCases := []TestCase{
		{"testdata/5kb", "de66260a16dd6add7e959103ffa7b72be109af92a537aa1ed4d931f258c246ff"},
		{"testdata/15kb", "9714865c29194f3ff85ec5e54b9dfa849f9a408cf4aebcf4e5983f0d41361534"},
		{"testdata/25kb", "b725db8e31a9f1d7b943e176a3d03a3dd82e2d95da52a3cb3f017ea520dd6448"},
	}

	for _, test := range testCases {
		actual := utils.GetSha256(test.input)
		if actual != test.expected {
			t.Errorf("utils.GetSha256(%q) = %v; want %v", test.input, actual, test.expected)
		}
	}
}
