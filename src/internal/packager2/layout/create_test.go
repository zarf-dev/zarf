// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/stretchr/testify/require"

	"github.com/zarf-dev/zarf/src/pkg/layout"
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"github.com/zarf-dev/zarf/src/types"
)

func TestSplitFile(t *testing.T) {
	tempDir := t.TempDir()
	inputFilename := filepath.Join(tempDir, "testfile.dat")
	testData := []byte("This is some data to test out that we can split files into chunks and put them back together")
	err := os.WriteFile(inputFilename, testData, helpers.ReadAllWriteUser)
	require.NoError(t, err)

	chunkSize := 30
	err = splitFile(inputFilename, chunkSize)
	require.NoError(t, err)

	_, err = os.Stat(inputFilename)
	require.True(t, os.IsNotExist(err))
	headerPath := inputFilename + ".part000"
	headerData, err := os.ReadFile(headerPath)
	require.NoError(t, err)
	var splitData types.ZarfSplitPackageData
	err = json.Unmarshal(headerData, &splitData)
	require.NoError(t, err)
	require.EqualValues(t, len(testData), splitData.Bytes)
	require.Greater(t, splitData.Count, 2, "make sure we actually split the file")


	assembledPath := filepath.Join(tempDir, "assembled.dat")
	err = assembleSplitTar(headerPath, assembledPath)
	require.NoError(t, err)
	reconstructed, err := os.ReadFile(assembledPath)
	require.Equal(t, testData, reconstructed)
	actualHash := sha256.Sum256(reconstructed)
	require.Equal(t, splitData.Sha256Sum, fmt.Sprintf("%x", actualHash))
}

func TestCreateSkeleton(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)

	lint.ZarfSchema = testutil.LoadSchema(t, "../../../../zarf.schema.json")

	opt := CreateOptions{}
	path, err := CreateSkeleton(ctx, "./testdata/zarf-skeleton-package", opt)
	require.NoError(t, err)

	pkgPath := layout.New(path)
	_, warnings, err := pkgPath.ReadZarfYAML()
	require.NoError(t, err)
	require.Empty(t, warnings)
	b, err := os.ReadFile(filepath.Join(pkgPath.Base, "checksums.txt"))
	require.NoError(t, err)
	expectedChecksum := `54f657b43323e1ebecb0758835b8d01a0113b61b7bab0f4a8156f031128d00f9 components/data-injections.tar
879bfe82d20f7bdcd60f9e876043cc4343af4177a6ee8b2660c304a5b6c70be7 components/files.tar
c497f1a56559ea0a9664160b32e4b377df630454ded6a3787924130c02f341a6 components/manifests.tar
fb7ebee94a4479bacddd71195030a483b0b0b96d4f73f7fcd2c2c8e0fce0c5c6 components/helm-charts.tar
`
	require.Equal(t, expectedChecksum, string(b))
}

func TestGetChecksum(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	files := map[string]string{
		"empty.txt":                "",
		"foo":                      "bar",
		"zarf.yaml":                "Zarf Yaml Data",
		"checksums.txt":            "Old Checksum Data",
		"nested/directory/file.md": "nested",
	}
	for k, v := range files {
		err := os.MkdirAll(filepath.Join(tmpDir, filepath.Dir(k)), 0o700)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(tmpDir, k), []byte(v), 0o600)
		require.NoError(t, err)
	}

	checksumContent, checksumHash, err := getChecksum(tmpDir)
	require.NoError(t, err)

	expectedContent := `233562de1a0288b139c4fa40b7d189f806e906eeb048517aeb67f34ac0e2faf1 nested/directory/file.md
e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 empty.txt
fcde2b2edba56bf408601fb721fe9b5c338d10ee429ea04fae5511b68fbf8fb9 foo
`
	require.Equal(t, expectedContent, checksumContent)
	require.Equal(t, "7c554cf67e1c2b50a1b728299c368cd56d53588300c37479623f29a52812ca3f", checksumHash)
}

func TestSignPackage(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "zarf.yaml")
	signedPath := filepath.Join(tmpDir, "zarf.yaml.sig")

	err := os.WriteFile(yamlPath, []byte("foobar"), 0o644)
	require.NoError(t, err)

	err = signPackage(tmpDir, "", "")
	require.NoError(t, err)
	require.NoFileExists(t, signedPath)

	err = signPackage(tmpDir, "./testdata/cosign.key", "wrongpassword")
	require.EqualError(t, err, "reading key: decrypt: encrypted: decryption failed")

	err = signPackage(tmpDir, "./testdata/cosign.key", "test")
	require.NoError(t, err)
	require.FileExists(t, signedPath)
}

func TestCreateReproducibleTarballFromDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	err := os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("hello world"), 0o600)
	require.NoError(t, err)
	tarPath := filepath.Join(t.TempDir(), "data.tar")

	err = createReproducibleTarballFromDir(tmpDir, "", tarPath, true)
	require.NoError(t, err)

	shaSum, err := helpers.GetSHA256OfFile(tarPath)
	require.NoError(t, err)
	require.Equal(t, "c09d17f612f241cdf549e5fb97c9e063a8ad18ae7a9f3af066332ed6b38556ad", shaSum)
}
