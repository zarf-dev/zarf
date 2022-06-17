package packager

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

var (
	ZarfFileName = "zarf.yaml"
)

const defaultZarfFile = `kind: ZarfPackageConfig
metadata:
  name: "init-package-git-server"

components:
  - name: git-server
    description: "Add Gitea for serving gitops-based clusters in an airgap"
    images:
      - gitea/gitea:1.16.8
    charts:
      - name: gitea
        url: https://dl.gitea.io/charts
        version: 5.0.8
        namespace: zarf
        valuesFiles:
          - gitea-values.yaml
    scripts:
      retry: true
      after:
        - "./zarf tools create-read-only-gitea-user"
`

func Generate(name, dir string) (string, error) {
	path, err := filepath.Abs(dir)
	if err != nil {
		return path, err
	}

	cdir := filepath.Join(path, name)
	if fi, err := os.Stat(cdir); err == nil && !fi.IsDir() {
		return cdir, errors.Errorf("file %s already exists and is not a directory", cdir)
	}

	files := []struct {
		path    string
		content []byte
	}{
		{
			// zarf.yaml
			path:    filepath.Join(cdir, ZarfFileName),
			content: []byte(fmt.Sprintf(defaultZarfFile, name)),
		},
	}

	for _, file := range files {
		if err := writeFile(file.path, file.content); err != nil {
			return cdir, err
		}
	}
	return cdir, nil

}

func writeFile(name string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
		return err
	}
	return ioutil.WriteFile(name, content, 0644)
}
