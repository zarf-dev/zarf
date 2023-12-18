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
	"github.com/stretchr/testify/require"
)

func TestPrivateHelm(t *testing.T) {
	t.Log("E2E: Private Helm")

	t.Run("zarf test helm success", func(t *testing.T) {
		t.Log("E2E: Private helm success")

		packagePath := filepath.Join("src", "test", "packages", "13-private-helm")
		cwd, _ := os.Getwd()
		repoPath := filepath.Join(cwd, packagePath, "repositories.yaml")

		os.Setenv("HELM_REPOSITORY_CONFIG", repoPath)
		// TODO this doesn't work, do we care about giving this option through CLI
		// _, _, err := e2e.Zarf("prepare", "find-images", packagePath, "--repository-config", repoPath)

		c := cluster.NewClusterOrDie()
		state, err := c.LoadZarfState()
		require.NoError(t, err)
		username := state.GitServer.PushUsername
		password := state.GitServer.PushPassword

		require.NoError(t, err)
		tunnelGit, err := c.Connect(cluster.ZarfGit)
		require.NoError(t, err)
		defer tunnelGit.Close()

		chartFilePath := filepath.Join("src", "test", "e2e", "podinfo-6.5.3.tgz")
		url := fmt.Sprintf("http://%s/api/packages/%s/helm/api/charts", tunnelGit.Endpoint(), username)

		// Open the file
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

		// Execute the request
		resp, err := client.Do(req)
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()

		// helm repo add  --username {username} --password {password} gitea-temp-repo http://127.0.0.1:41861/api/packages/zarf-git-user/helm
		// helm repo add   gitea-temp-repo http://127.0.0.1:41861/api/packages/zarf-git-user/helm
		// helm cm-push ./{chart_file}.tgz {repo}

	})
}
