// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/defenseunicorns/pkg/helpers"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/packager/creator"
	"github.com/defenseunicorns/zarf/src/pkg/packager/filters"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/fsnotify/fsnotify"
	"github.com/monochromegane/go-gitignore"
)

// DevDeployOpts provides options to configure the behavior of dev deploy.
type DevDeployOpts struct {
	Watch bool
}

// DevDeploy creates + deploys a package in one shot
func (p *Packager) DevDeploy(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

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

	if err := p.cfg.Pkg.Validate(); err != nil {
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
// It watches one or more filepaths and performs a DevDeploy() whenever a change is detected.
//
// TODO:
//
// - Improve DX for --watch (especially handling errors and reducing noisy output)
//
// - Allow component-level reloads
func (p *Packager) WatchAndReload(filepaths ...string) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("creating a new watcher: %w", err)
	}
	defer w.Close()

	basePath := p.cfg.CreateOpts.BaseDir
	gitignorePath := filepath.Join(basePath, ".gitignore")

	ignoreMatcher, err := gitignore.NewGitIgnore(gitignorePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			ignoreMatcher = gitignore.NewGitIgnoreFromReader("", strings.NewReader(""))
			message.Info("No .gitignore file found; watching without ignoring any files...")
		} else {
			return fmt.Errorf("loading .gitignore: %w", err)
		}
	}

	filepaths = append(filepaths, basePath)
	for _, path := range filepaths {
		if err := seedWatcher(w, path, ignoreMatcher); err != nil {
			return err
		}
	}

	message.Info("Watching files for zarf dev deploy...")
	go p.devDeployLoop(w, gitignorePath, ignoreMatcher)

	select {}
}

func (p *Packager) devDeployLoop(w *fsnotify.Watcher, gitignorePath string, ignoreMatcher gitignore.IgnoreMatcher) {
	debounceDuration := 1 * time.Second
	var currentCtx context.Context
	var cancel context.CancelFunc = func() {}
	var debounceTimer *time.Timer

	for {
		select {
		case err, ok := <-w.Errors:
			if !ok {
				message.Warn("Error channel unexpectedly closed")
				cancel()
				return
			}
			message.Warn(err.Error())

		case e, ok := <-w.Events:
			if !ok {
				message.Warn("Events channel unexpectedly closed")
				cancel()
				return
			}

			if e.Name == gitignorePath && e.Has(fsnotify.Write) {
				newIgnoreMatcher, err := gitignore.NewGitIgnore(gitignorePath)
				if err != nil {
					message.Warnf("Failed to reload .gitignore: %s", err.Error())
					continue
				}
				ignoreMatcher = newIgnoreMatcher
				message.Info(".gitignore reloaded")
			}

			if !e.Has(fsnotify.Write) || ignoreMatcher.Match(e.Name, false) {
				continue
			}

			message.Infof("Detected Write event: %s", e.Name)
			if debounceTimer != nil {
				debounceTimer.Stop()
			}

			cancel()
			currentCtx, cancel = context.WithCancel(context.Background())

			debounceTimer = time.AfterFunc(debounceDuration, func() {
				if err := p.DevDeploy(currentCtx); err != nil {
					message.Warn(err.Error())
				} else {
					message.Success("Deployment successful. Watching for further changes...")
				}
			})
		}
	}
}

// seedWatcher configures a fsnotify.Watcher to monitor a specified path and all its subdirectories.
// It adheres to .gitignore rules, excluding any directories that match these patterns.
// It only adds directories to the watcher, aligning with fsnotify's guidance to avoid
// watching individual files that may frequently undergo temporary or insignificant changes (e.g., intermediate saves by text editors).
//
// For more information: https://github.com/fsnotify/fsnotify?tab=readme-ov-file#watching-a-file-doesnt-work-well
//
// Note: fsnotify does not natively support recursive directory watching. As a workaround,
// seedWatcher traverses the directory tree and adds each subdirectory to the watcher.
//
// For more information: https://github.com/fsnotify/fsnotify/issues/18
func seedWatcher(w *fsnotify.Watcher, path string, ignoreMatcher gitignore.IgnoreMatcher) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	return filepath.WalkDir(absPath, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(absPath, p)
		if err != nil {
			return err
		}

		if d.IsDir() && ignoreMatcher.Match(relPath, true) {
			return filepath.SkipDir
		}

		if d.IsDir() {
			return w.Add(p)
		}

		return nil
	})
}
