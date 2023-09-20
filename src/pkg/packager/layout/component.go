// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/mholt/archiver/v3"
)

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

type Components struct {
	Base     string
	Dirs     map[string]*ComponentPaths
	Tarballs map[string]string
}

func (c *Components) Archive(component types.ZarfComponent) (err error) {
	name := component.Name
	if _, ok := c.Dirs[name]; !ok {
		return nil
	}
	base := c.Dirs[name].Base
	_ = os.RemoveAll(c.Dirs[name].Temp)
	size, err := utils.GetDirSize(base)
	if err != nil {
		return err
	}
	if size > 0 {
		tb := fmt.Sprintf("%s.tar", base)
		message.Debugf("Archiving %q", name)
		if err := archiver.Archive([]string{base}, tb); err != nil {
			return err
		}
		if c.Tarballs == nil {
			c.Tarballs = make(map[string]string)
		}
		c.Tarballs[name] = tb
	} else {
		message.Debugf("Component %q is empty, skipping archiving", name)
	}

	delete(c.Dirs, name)
	return os.RemoveAll(base)
}

func (c *Components) Unarchive(component types.ZarfComponent) (err error) {
	name := component.Name
	tb, ok := c.Tarballs[name]
	if !ok {
		return nil
	}

	if utils.InvalidPath(tb) {
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
	if !utils.InvalidPath(cs.Base) {
		message.Debugf("Component %q already unarchived", name)
		return nil
	}

	message.Debugf("Unarchiving %q", tb)
	if err := archiver.Unarchive(tb, c.Base); err != nil {
		return err
	}
	return os.Remove(tb)
}

func (c *Components) Create(component types.ZarfComponent) (cl *ComponentPaths, err error) {
	if err = utils.CreateDirectory(c.Base, 0700); err != nil {
		return nil, err
	}

	name := component.Name
	base := filepath.Join(c.Base, name)

	if err = utils.CreateDirectory(base, 0700); err != nil {
		return nil, err
	}

	cl = &ComponentPaths{
		Base: base,
	}

	cl.Temp = filepath.Join(base, TempDir)
	if err = utils.CreateDirectory(cl.Temp, 0700); err != nil {
		return nil, err
	}

	if len(component.Files) > 0 {
		cl.Files = filepath.Join(base, FilesDir)
		if err = utils.CreateDirectory(cl.Files, 0700); err != nil {
			return nil, err
		}
	}

	if len(component.Charts) > 0 {
		cl.Charts = filepath.Join(base, ChartsDir)
		if err = utils.CreateDirectory(cl.Charts, 0700); err != nil {
			return nil, err
		}
		for _, chart := range component.Charts {
			cl.Values = filepath.Join(base, ValuesDir)
			if len(chart.ValuesFiles) > 0 {
				if err = utils.CreateDirectory(cl.Values, 0700); err != nil {
					return nil, err
				}
				break
			}
		}
	}

	if len(component.Repos) > 0 {
		cl.Repos = filepath.Join(base, ReposDir)
		if err = utils.CreateDirectory(cl.Repos, 0700); err != nil {
			return nil, err
		}
	}

	if len(component.Manifests) > 0 {
		cl.Manifests = filepath.Join(base, ManifestsDir)
		if err = utils.CreateDirectory(cl.Manifests, 0700); err != nil {
			return nil, err
		}
	}

	if len(component.DataInjections) > 0 {
		cl.DataInjections = filepath.Join(base, DataInjectionsDir)
		if err = utils.CreateDirectory(cl.DataInjections, 0700); err != nil {
			return nil, err
		}
	}

	if c.Dirs == nil {
		c.Dirs = make(map[string]*ComponentPaths)
	}

	c.Dirs[name] = cl
	return cl, nil
}
