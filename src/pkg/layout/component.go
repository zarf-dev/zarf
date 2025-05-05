// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package layout contains functions for interacting with Zarf's package layout on disk.
package layout

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/mholt/archiver/v3"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/logger"
)

// ComponentPaths contains paths for a component.
type ComponentPaths struct {
	Base           string
	Temp           string
	Files          string
	Charts         string
	Values         string
	Repos          string
	Manifests      string
	DataInjections string
}

// Components contains paths for components.
type Components struct {
	Base     string
	Dirs     map[string]*ComponentPaths
	Tarballs map[string]string
}

// ErrNotLoaded is returned when a path is not loaded.
var ErrNotLoaded = fmt.Errorf("not loaded")

// Archive archives a component.
func (c *Components) Archive(ctx context.Context, component v1alpha1.ZarfComponent, cleanupTemp bool) error {
	l := logger.From(ctx)
	name := component.Name
	if _, ok := c.Dirs[name]; !ok {
		return &fs.PathError{
			Op:   "check dir map for",
			Path: name,
			Err:  ErrNotLoaded,
		}
	}
	base := c.Dirs[name].Base
	if cleanupTemp {
		err := os.RemoveAll(c.Dirs[name].Temp)
		if err != nil {
			return err
		}
	}
	size, err := helpers.GetDirSize(base)
	if err != nil {
		return err
	}
	if size > 0 {
		tb := fmt.Sprintf("%s.tar", base)
		l.Debug("archiving component", "name", name)
		if err := helpers.CreateReproducibleTarballFromDir(base, name, tb); err != nil {
			return err
		}
		if c.Tarballs == nil {
			c.Tarballs = make(map[string]string)
		}
		c.Tarballs[name] = tb
	} else {
		l.Debug("component is empty, skipping archiving", "name", name)
	}

	delete(c.Dirs, name)
	return os.RemoveAll(base)
}

// Unarchive unarchives a component.
func (c *Components) Unarchive(ctx context.Context, component v1alpha1.ZarfComponent) error {
	l := logger.From(ctx)
	name := component.Name
	tb, ok := c.Tarballs[name]
	if !ok {
		return &fs.PathError{
			Op:   "check tarball map for",
			Path: name,
			Err:  ErrNotLoaded,
		}
	}

	if helpers.InvalidPath(tb) {
		return &fs.PathError{
			Op:   "stat",
			Path: tb,
			Err:  fs.ErrNotExist,
		}
	}

	cs := &ComponentPaths{
		Base: filepath.Join(c.Base, name),
	}
	if len(component.Files) > 0 {
		cs.Files = filepath.Join(cs.Base, FilesDir)
	}
	if len(component.Charts) > 0 {
		cs.Charts = filepath.Join(cs.Base, ChartsDir)
		for _, chart := range component.Charts {
			if len(chart.ValuesFiles) > 0 {
				cs.Values = filepath.Join(cs.Base, ValuesDir)
				break
			}
		}
	}
	if len(component.Repos) > 0 {
		cs.Repos = filepath.Join(cs.Base, ReposDir)
	}
	if len(component.Manifests) > 0 {
		cs.Manifests = filepath.Join(cs.Base, ManifestsDir)
	}
	if len(component.DataInjections) > 0 {
		cs.DataInjections = filepath.Join(cs.Base, DataInjectionsDir)
	}
	if c.Dirs == nil {
		c.Dirs = make(map[string]*ComponentPaths)
	}
	c.Dirs[name] = cs
	delete(c.Tarballs, name)

	// if the component is already unarchived, skip
	if !helpers.InvalidPath(cs.Base) {
		l.Debug("component already unarchived", "component", name)
		return nil
	}

	// TODO(mkcp): Bring in context and port to logger
	l.Debug("unarchiving", "component", filepath.Base(tb))
	// TODO(mkcp): See https://github.com/zarf-dev/zarf/issues/3051
	if err := archiver.Unarchive(tb, c.Base); err != nil {
		return err
	}
	return os.Remove(tb)
}

// Create creates a new component directory structure.
func (c *Components) Create(component v1alpha1.ZarfComponent) (*ComponentPaths, error) {
	name := component.Name

	_, ok := c.Tarballs[name]
	if ok {
		return nil, &fs.PathError{
			Op:   "create component paths",
			Path: name,
			Err:  fmt.Errorf("component tarball for %q exists, use Unarchive instead", name),
		}
	}

	if err := helpers.CreateDirectory(c.Base, helpers.ReadWriteExecuteUser); err != nil {
		return nil, err
	}

	base := filepath.Join(c.Base, name)

	if err := helpers.CreateDirectory(base, helpers.ReadWriteExecuteUser); err != nil {
		return nil, err
	}

	cp := &ComponentPaths{
		Base: base,
	}

	cp.Temp = filepath.Join(base, TempDir)
	if err := helpers.CreateDirectory(cp.Temp, helpers.ReadWriteExecuteUser); err != nil {
		return nil, err
	}

	if len(component.Files) > 0 {
		cp.Files = filepath.Join(base, FilesDir)
		if err := helpers.CreateDirectory(cp.Files, helpers.ReadWriteExecuteUser); err != nil {
			return nil, err
		}
	}

	if len(component.Charts) > 0 {
		cp.Charts = filepath.Join(base, ChartsDir)
		if err := helpers.CreateDirectory(cp.Charts, helpers.ReadWriteExecuteUser); err != nil {
			return nil, err
		}
		for _, chart := range component.Charts {
			cp.Values = filepath.Join(base, ValuesDir)
			if len(chart.ValuesFiles) > 0 {
				if err := helpers.CreateDirectory(cp.Values, helpers.ReadWriteExecuteUser); err != nil {
					return nil, err
				}
				break
			}
		}
	}

	if len(component.Repos) > 0 {
		cp.Repos = filepath.Join(base, ReposDir)
		if err := helpers.CreateDirectory(cp.Repos, helpers.ReadWriteExecuteUser); err != nil {
			return nil, err
		}
	}

	if len(component.Manifests) > 0 {
		cp.Manifests = filepath.Join(base, ManifestsDir)
		if err := helpers.CreateDirectory(cp.Manifests, helpers.ReadWriteExecuteUser); err != nil {
			return nil, err
		}
	}

	if len(component.DataInjections) > 0 {
		cp.DataInjections = filepath.Join(base, DataInjectionsDir)
		if err := helpers.CreateDirectory(cp.DataInjections, helpers.ReadWriteExecuteUser); err != nil {
			return nil, err
		}
	}

	if c.Dirs == nil {
		c.Dirs = make(map[string]*ComponentPaths)
	}

	c.Dirs[name] = cp
	return cp, nil
}
