// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic utility functions.
package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/defenseunicorns/pkg/helpers"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TestNetworkSuite struct {
	suite.Suite
	*require.Assertions
}

func (suite *TestNetworkSuite) SetupSuite() {
	suite.Assertions = require.New(suite.T())
}

func (suite *TestNetworkSuite) Test_0_parseChecksum() {
	// zarf prepare sha256sum .adr-dir
	adr := "https://raw.githubusercontent.com/defenseunicorns/zarf/main/.adr-dir"
	sum := "930f4d5a191812e57b39bd60fca789ace07ec5acd36d63e1047604c8bdf998a3"
	url := adr + "@" + sum
	uri, checksum, err := parseChecksum(url)
	suite.NoError(err)
	suite.Equal(adr, uri)
	suite.Equal(sum, checksum)

	url = adr + "?foo=bar@" + sum
	uri, checksum, err = parseChecksum(url)
	suite.NoError(err)
	suite.Equal(adr+"?foo=bar", uri)
	suite.Equal(sum, checksum)

	url = "https://user:pass@hello.world?foo=bar"
	uri, checksum, err = parseChecksum(url)
	suite.NoError(err)
	suite.Equal("https://user:pass@hello.world?foo=bar", uri)
	suite.Equal("", checksum)

	url = "https://user:pass@hello.world?foo=bar@" + sum
	uri, checksum, err = parseChecksum(url)
	suite.NoError(err)
	suite.Equal("https://user:pass@hello.world?foo=bar", uri)
	suite.Equal(sum, checksum)
}

func (suite *TestNetworkSuite) Test_1_DownloadToFile() {
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
	content, err := os.ReadFile(path)
	suite.NoError(err)
	suite.Contains(string(content), "adr")

	check, err := helpers.GetSHA256OfFile(path)
	suite.NoError(err)
	suite.Equal(sum, check)

	url = adr + "@" + "badsha"
	path = filepath.Join(tmp, ".adr-dir.bad")
	suite.Error(DownloadToFile(url, path, ""))

	url = adr + "?foo=bar@" + sum
	path = filepath.Join(tmp, ".adr-dir.good")
	suite.NoError(DownloadToFile(url, path, ""))
	suite.FileExists(path)
}

func TestNetwork(t *testing.T) {
	message.SetLogLevel(message.DebugLevel)
	suite.Run(t, new(TestNetworkSuite))
}
