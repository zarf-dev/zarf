package packager

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

var (
	ZarfFileName   = "zarf.yaml"
	ValuesFileName = "gitea-values.yaml"
	ReadMeFileName = "README.md"
)

const defaultZarfFile = `kind: ZarfPackageConfig
metadata:
  name: "%s"

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

const defaultValuesFile = `--- # Values file for the Zarf package: "%s"
persistence:
  storageClass: "###ZARF_STORAGE_CLASS###"
gitea:
  admin:
    username: "zarf-git-user"
    password: "###ZARF_GIT_AUTH_PUSH###"
    email: "zarf@localhost"
  config:
    APP_NAME: "Zarf Gitops Service"
    server:
      DISABLE_SSH: true
      OFFLINE_MODE: true
    database:
      DB_TYPE: sqlite3
      # Note that the init script checks to see if the IP & port of the database service is accessible, so make sure you set those to something that resolves as successful (since sqlite uses files on disk setting the port & ip won't affect the running of gitea).
      HOST: zarf-docker-registry.zarf.svc.cluster.local:5000
    security:
      INSTALL_LOCK: true
    service:
      DISABLE_REGISTRATION: true
    repository:
      ENABLE_PUSH_CREATE_USER: true
      FORCE_PRIVATE: true
resources:
  requests:
    cpu: "200m"
    memory: "512Mi"
  limits:
    cpu: "1"
    memory: "2Gi"

memcached:
  enabled: false

postgresql:
  enabled: false
`

const defaultReadmeFile = `## Zarf Git Server

The "%s" Zarf package contains the Zarf Git Server to enable more advanced gitops-based deployments such as the [gitops-data](../../examples/gitops-data/README.md) example.
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

		{
			// gitea-values.yaml
			path:    filepath.Join(cdir, ValuesFileName),
			content: []byte(fmt.Sprintf(defaultValuesFile, name)),
		},

		{
			// README.md
			path:    filepath.Join(cdir, ReadMeFileName),
			content: []byte(fmt.Sprintf(defaultReadmeFile, name)),
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
