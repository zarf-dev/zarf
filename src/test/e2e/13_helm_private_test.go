package test

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/defenseunicorns/zarf/src/pkg/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/repo"
)

func TestPrivateHelm(t *testing.T) {
	t.Log("E2E: Private Helm")

	t.Run("zarf test helm success", func(t *testing.T) {
		t.Log("E2E: Private helm success")

		dirPath := filepath.Join("src", "test", "packages", "13-private-helm")
		zarfPackagePath := filepath.Join(dirPath, "zarf.yaml")

		c, err := cluster.NewCluster()
		require.NoError(t, err)
		state, err := c.LoadZarfState()
		require.NoError(t, err)
		username := state.GitServer.PushUsername
		password := state.GitServer.PushPassword

		require.NoError(t, err)
		tunnelGit, err := c.Connect(cluster.ZarfGit)
		require.NoError(t, err)
		defer tunnelGit.Close()

		repoFile := repo.NewFile()
		baseURL := fmt.Sprintf("http://%s", tunnelGit.Endpoint())
		chartURL := fmt.Sprintf("%s/api/packages/zarf-git-user/helm", baseURL)
		Entry := repo.Entry{
			Name:     "temp_entry",
			Username: username,
			Password: password,
			URL:      chartURL,
		}
		repoFile.Add(&Entry)
		tempDir := t.TempDir()
		repoPath := filepath.Join(tempDir, "repositories.yaml")
		os.Setenv("HELM_REPOSITORY_CONFIG", repoPath)
		utils.WriteYaml(repoPath, repoFile, 0600)

		createHelmChartInGitea(t, baseURL, username, password)

		var zarfPackage types.ZarfPackage
		utils.ReadYaml(zarfPackagePath, &zarfPackage)
		zarfPackage.Components[0].Charts[0].URL = chartURL
		newPackagePath := filepath.Join(tempDir, "zarf.yaml")
		utils.WriteYaml(newPackagePath, zarfPackage, 0600)
		_, _, err = e2e.Zarf("prepare", "find-images", tempDir)
		require.NoError(t, err)

		// helm repo add  --username {username} --password {password} gitea-temp-repo http://127.0.0.1:41861/api/packages/zarf-git-user/helm
		// helm repo add   gitea-temp-repo http://127.0.0.1:41861/api/packages/zarf-git-user/helm
		// helm cm-push ./{chart_file}.tgz {repo}

	})
}

func createHelmChartInGitea(t *testing.T, baseURL string, username string, password string) {
	chartFilePath := filepath.Join("src", "test", "e2e", "podinfo-6.5.3.tgz")
	url := fmt.Sprintf("%s/api/packages/%s/helm/api/charts", baseURL, username)

	file, err := os.Open(chartFilePath)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", chartFilePath)
	require.NoError(t, err)
	_, err = io.Copy(part, file)
	require.NoError(t, err)
	writer.Close()

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		panic(err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.SetBasicAuth(username, password)

	client := &http.Client{}

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
}
