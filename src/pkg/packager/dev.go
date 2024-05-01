// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/defenseunicorns/pkg/helpers"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/packager/validate"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/packager/creator"
	"github.com/defenseunicorns/zarf/src/pkg/packager/filters"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/fsnotify/fsnotify"
)

// DevDeployOpts provides options to configure the behavior of dev deploy.
type DevDeployOpts struct {
	Watch bool
}

// DevDeploy creates + deploys a package in one shot
func (p *Packager) DevDeploy() error {
	config.CommonOptions.Confirm = true
	p.cfg.CreateOpts.SkipSBOM = !p.cfg.CreateOpts.NoYOLO

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	if err := os.Chdir(p.cfg.CreateOpts.BaseDir); err != nil {
		return fmt.Errorf("unable to access directory %q: %w", p.cfg.CreateOpts.BaseDir, err)
	}

	pc := creator.NewPackageCreator(p.cfg.CreateOpts, cwd)

	if err := helpers.CreatePathAndCopy(layout.ZarfYAML, p.layout.ZarfYAML); err != nil {
		return err
	}

	p.cfg.Pkg, p.warnings, err = pc.LoadPackageDefinition(p.layout)
	if err != nil {
		return err
	}

	filter := filters.Combine(
		filters.ByLocalOS(runtime.GOOS),
		filters.ForDeploy(p.cfg.PkgOpts.OptionalComponents, false),
	)
	p.cfg.Pkg.Components, err = filter.Apply(p.cfg.Pkg)
	if err != nil {
		return err
	}

	if err := validate.Run(p.cfg.Pkg); err != nil {
		return fmt.Errorf("unable to validate package: %w", err)
	}

	if err := p.populatePackageVariableConfig(); err != nil {
		return fmt.Errorf("unable to set the active variables: %w", err)
	}

	// If building in yolo mode, strip out all images and repos
	if !p.cfg.CreateOpts.NoYOLO {
		for idx := range p.cfg.Pkg.Components {
			p.cfg.Pkg.Components[idx].Images = []string{}
			p.cfg.Pkg.Components[idx].Repos = []string{}
		}
	}

	if err := pc.Assemble(p.layout, p.cfg.Pkg.Components, p.cfg.Pkg.Metadata.Architecture); err != nil {
		return err
	}

	// cd back
	if err := os.Chdir(cwd); err != nil {
		return err
	}

	message.HeaderInfof("📦 PACKAGE DEPLOY %s", p.cfg.Pkg.Metadata.Name)

	p.connectStrings = make(types.ConnectStrings)

	if !p.cfg.CreateOpts.NoYOLO {
		p.cfg.Pkg.Metadata.YOLO = true
	} else {
		p.hpaModified = false
		// Reset registry HPA scale down whether an error occurs or not
		defer p.resetRegistryHPA()
	}

	// Get a list of all the components we are deploying and actually deploy them
	deployedComponents, err := p.deployComponents()
	if err != nil {
		return err
	}
	if len(deployedComponents) == 0 {
		message.Warn("No components were selected for deployment.  Inspect the package to view the available components and select components interactively or by name with \"--components\"")
	}

	// Notify all the things about the successful deployment
	message.Successf("Zarf dev deployment complete")

	message.HorizontalRule()
	message.Title("Next steps:", "")

	message.ZarfCommand("package inspect %s", p.cfg.Pkg.Metadata.Name)

	return nil
}

// WatchAndReload enables a hot reloading workflow with `zarf dev deploy --watch`.
//
// It watches one or more filepaths and performs a DevDeploy() whenever a change is detected.
//
// Note: Watching individual files is not supported because of various issues
// where files are frequently renamed, such as editors saving them.
func (p *Packager) WatchAndReload(filepaths ...string) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("creating a new watcher: %w", err)
	}
	defer w.Close()

	go p.devDeployLoop(w)

	filepaths = append(filepaths, p.cfg.CreateOpts.BaseDir)
	for _, path := range filepaths {
		if err := seedWatcher(w, path); err != nil {
			return err
		}
	}

	message.Info("Watching files for zarf dev deploy...")

	select {}
}

func (p *Packager) devDeployLoop(w *fsnotify.Watcher) {
	var debounceTimer *time.Timer
	debounceDuration := 2 * time.Second

	for {
		select {
		case err, ok := <-w.Errors:
			if !ok {
				message.Warn("Error channel unexpectedly closed")
				return
			}
			message.Warn(err.Error())

		case e, ok := <-w.Events:
			if !ok {
				message.Warn("Events channel unexpectedly closed")
				return
			}

			if e.Has(fsnotify.Write) {
				message.Infof("Detected Write event: %s", e.Name)

				if debounceTimer != nil {
					debounceTimer.Stop()
				}

				debounceTimer = time.AfterFunc(debounceDuration, func() {
					if err := p.DevDeploy(); err != nil {
						message.WarnErrf(err, "Error deploying changes made to: %s", e.Name)
						message.Info("Watching files for further changes...")
					} else {
						message.Success("Deployment successful. Watching files for further changes...")
					}
				})
			}
		}
	}
}

// seedWatcher adds path and all of its subdirectories to the watcher.
//
// This is needed because fsnotify.Watcher does not support recursive watch: https://github.com/fsnotify/fsnotify/issues/18
func seedWatcher(w *fsnotify.Watcher, path string) error {
	return filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			err = w.Add(p)
			if err != nil {
				return fmt.Errorf("adding filepath %q to watcher: %w", p, err)
			}
		}
		return nil
	})
}
