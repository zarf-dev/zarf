// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packages provides api functions for managing Zarf packages.
package packages

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	globalConfig "github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/internal/api/common"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/packager"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

// DeployPackage deploys a package to the Zarf cluster.
func DeployPackage(w http.ResponseWriter, r *http.Request) {
	config := types.PackagerConfig{}

	type DeployPayload struct {
		DeployOpts types.ZarfDeployOptions `json:"deployOpts"`
		InitOpts   *types.ZarfInitOptions  `json:"initOpts,omitempty"`
	}

	var body DeployPayload

	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		message.ErrorWebf(err, w, "Unable to decode the request to deploy the cluster")
		return
	}

	// Check if init options is empty
	if body.InitOpts != nil {
		config.InitOpts = *body.InitOpts
		config.DeployOpts = body.DeployOpts
		initPackageName := packager.GetInitPackageName("")
		config.DeployOpts.PackagePath = initPackageName
		// Now find the init package like in src/cmd/initialize.go
		var err error
		if config.DeployOpts.PackagePath, err = findInitPackage(initPackageName); err != nil {
			message.ErrorWebf(err, w, fmt.Sprintf("Unable to find the %s to deploy the cluster", initPackageName))
			return
		}
	} else {
		config.DeployOpts = body.DeployOpts
	}

	globalConfig.CommonOptions.Confirm = true

	pkgClient := packager.NewOrDie(&config)
	defer pkgClient.ClearTempPaths()

	if err := pkgClient.Deploy(); err != nil {
		message.ErrorWebf(err, w, "Unable to deploy the zarf package to the cluster")
		return
	}

	common.WriteJSONResponse(w, true, http.StatusCreated)
}

// Taken from src/cmd/initialize.go
func findInitPackage(initPackageName string) (string, error) {
	// First, look for the init package in the current working directory
	if !utils.InvalidPath(initPackageName) {
		return initPackageName, nil
	}

	// Next, look for the init package in the executable directory
	executablePath, err := utils.GetFinalExecutablePath()
	if err != nil {
		return "", err
	}
	executableDir := path.Dir(executablePath)
	if !utils.InvalidPath(filepath.Join(executableDir, initPackageName)) {
		return filepath.Join(executableDir, initPackageName), nil
	}

	// Create the cache directory if it doesn't exist
	if utils.InvalidPath(globalConfig.GetAbsCachePath()) {
		if err := os.MkdirAll(globalConfig.GetAbsCachePath(), 0755); err != nil {
			return "", fmt.Errorf(strings.ToLower(lang.CmdInitErrUnableCreateCache), globalConfig.GetAbsCachePath())
		}
	}

	// Next, look in the cache directory
	if !utils.InvalidPath(filepath.Join(globalConfig.GetAbsCachePath(), initPackageName)) {
		return filepath.Join(globalConfig.GetAbsCachePath(), initPackageName), nil
	}

	// Otherwise return an error
	return "", errors.New("unable to find the init package")
}
