// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic helper functions.
package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
)

type TestNetworkSuite struct {
	suite.Suite
	urls testURLs
}

type testURLs struct {
	good  []string
	oci   []string
	paths []string
}

func (suite *TestNetworkSuite) SetupSuite() {
	suite.urls.good = []string{
		"https://zarf.dev",
		"https://docs.zarf.dev",
		"https://zarf.dev/docs",
		"https://defenseunicorns.com",
		"https://google.com",
	}
	suite.urls.oci = []string{
		"oci://zarf.dev",
		"oci://defenseunicorns.com",
		"oci://google.com",
	}
	suite.urls.paths = []string{
		"./hello.txt",
		"../hello.txt",
		"/tmp/hello.txt",
	}
}

func (suite *TestNetworkSuite) Test_0_IsURL() {
	all := append(suite.urls.good, suite.urls.oci...)
	for _, url := range all {
		suite.True(IsURL(url), "Expected %s to be a valid URL", url)
	}
	for _, url := range suite.urls.paths {
		suite.False(IsURL(url), "Expected %s to be an invalid URL", url)
	}
}

func (suite *TestNetworkSuite) Test_1_IsOCIURL() {
	for _, url := range suite.urls.good {
		suite.False(IsOCIURL(url), "Expected %s to be an invalid OCI URL", url)
	}
	for _, url := range suite.urls.oci {
		suite.True(IsOCIURL(url), "Expected %s to be a valid OCI URL", url)
	}
	for _, url := range suite.urls.paths {
		suite.False(IsOCIURL(url), "Expected %s to be an invalid OCI URL", url)
	}
}

func (suite *TestNetworkSuite) Test_2_DoHostnamesMatch() {
	b, err := DoHostnamesMatch("https://zarf.dev", "https://zarf.dev")
	suite.NoError(err)
	suite.True(b)
	b, err = DoHostnamesMatch("https://zarf.dev", "https://docs.zarf.dev")
	suite.NoError(err)
	suite.False(b)
	b, err = DoHostnamesMatch("https://zarf.dev", "https://zarf.dev/docs")
	suite.NoError(err)
	suite.True(b)
	b, err = DoHostnamesMatch("https://zarf.dev", "https://user:pass@zarf.dev")
	suite.NoError(err)
	suite.True(b)
	b, err = DoHostnamesMatch("https://zarf.dev", "")
	suite.NoError(err)
	suite.False(b)
	b, err = DoHostnamesMatch("", "https://zarf.dev")
	suite.NoError(err)
	suite.False(b)
}

func (suite *TestNetworkSuite) Test_3_Fetch() {
	readme := "https://raw.githubusercontent.com/defenseunicorns/zarf/main/README.md"
	body := Fetch(readme)
	defer body.Close()
	suite.NotNil(body)
}

func (suite *TestNetworkSuite) Test_4_DownloadToFile() {
	readme := "https://raw.githubusercontent.com/defenseunicorns/zarf/main/README.md"
	tmp := suite.T().TempDir()
	path := filepath.Join(tmp, "README.md")
	suite.NoError(DownloadToFile(readme, path, ""))
	suite.FileExists(path)

	path = filepath.Join(tmp, "README.md.bad")
	bad := "https://raw.githubusercontent.com/defenseunicorns/zarf/main/README.md.bad"
	suite.Error(DownloadToFile(bad, path, ""))

	// zarf prepare sha256sum .adr-dir
	path = filepath.Join(tmp, ".adr-dir")
	sum := "930f4d5a191812e57b39bd60fca789ace07ec5acd36d63e1047604c8bdf998a3"
	adr := "https://raw.githubusercontent.com/defenseunicorns/zarf/main/.adr-dir"
	url := adr + "@" + sum
	err := DownloadToFile(url, path, "")
	suite.NoError(err)
	suite.FileExists(path)
	content, err := os.ReadFile(filepath.Join(tmp, ".adr-dir"))
	suite.NoError(err)
	suite.Contains(string(content), "adr")

	check, err := GetSHA256OfFile(filepath.Join(tmp, ".adr-dir"))
	suite.NoError(err)
	suite.Equal(sum, check)

	url = adr + "@" + "badsha"
	path = filepath.Join(tmp, ".adr-dir.bad")
	suite.NoError(err)
	suite.Error(DownloadToFile(url, path, ""))
}

func TestNetwork(t *testing.T) {
	suite.Run(t, new(TestNetworkSuite))
}
