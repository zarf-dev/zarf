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
	base := c.Dirs[name].Base
	_ = os.RemoveAll(c.Dirs[name].Temp)
	size, err := utils.GetDirSize(base)
	if err != nil {
		return err
	}
	if size > 0 {
		tb := fmt.Sprintf("%s.tar", base)
		message.Debugf("Archiving %q", base)
		if err := archiver.Archive([]string{base}, tb); err != nil {
			return err
		}
		if c.Tarballs == nil {
			c.Tarballs = make(map[string]string)
		}
		c.Tarballs[name] = tb
	} else {
		message.Debugf("Component %q is empty, skipping archiving", component.Name)
	}

	delete(c.Dirs, name)
	return os.RemoveAll(base)
}

func (c *Components) Unarchive(component types.ZarfComponent) (err error) {
	name := component.Name
	tb := c.Tarballs[name]

	if utils.InvalidPath(tb) {
		return &fs.PathError{
			Op:   "stat",
			Path: tb,
			Err:  fs.ErrNotExist,
		}
	}

	defer os.Remove(tb)
	cs := &ComponentPaths{
		Base: filepath.Join(c.Base, name),
	}
	if err := archiver.Unarchive(tb, cs.Base); err != nil {
		return err
	}
	if len(component.Files) > 0 {
		cs.Files = filepath.Join(cs.Base, "files")
	}
	if len(component.Charts) > 0 {
		cs.Charts = filepath.Join(cs.Base, "charts")
		for _, chart := range component.Charts {
			if len(chart.ValuesFiles) > 0 {
				cs.Values = filepath.Join(cs.Base, "charts", chart.Name)
				break
			}
		}
	}
	if len(component.Repos) > 0 {
		cs.Repos = filepath.Join(cs.Base, "repos")
	}
	if len(component.Manifests) > 0 {
		cs.Manifests = filepath.Join(cs.Base, "manifests")
	}
	if len(component.DataInjections) > 0 {
		cs.DataInjections = filepath.Join(cs.Base, "data-injections")
	}
	c.Dirs[name] = cs
	delete(c.Tarballs, name)
	return nil
}

func (c *Components) Create(component types.ZarfComponent) (cl *ComponentPaths, err error) {
	if err = utils.CreateDirectory(c.Base, 0700); err != nil {
		return
	}

	name := component.Name
	base := filepath.Join(c.Base, name)

	if err = utils.CreateDirectory(base, 0700); err != nil {
		return
	}

	cl = &ComponentPaths{
		Base: base,
	}

	if len(component.Files) > 0 {
		cl.Files = filepath.Join(base, "files")
		if err = utils.CreateDirectory(cl.Files, 0700); err != nil {
			return
		}
	}

	if len(component.Charts) > 0 {
		cl.Charts = filepath.Join(base, "charts")
		if err = utils.CreateDirectory(cl.Charts, 0700); err != nil {
			return
		}
		for _, chart := range component.Charts {
			cl.Values = filepath.Join(base, "values")
			if len(chart.ValuesFiles) > 0 {
				if err = utils.CreateDirectory(cl.Values, 0700); err != nil {
					return
				}
				break
			}
		}
	}

	if len(component.Repos) > 0 {
		cl.Repos = filepath.Join(base, "repos")
		if err = utils.CreateDirectory(cl.Repos, 0700); err != nil {
			return
		}
	}

	if len(component.Manifests) > 0 {
		cl.Manifests = filepath.Join(base, "manifests")
		if err = utils.CreateDirectory(cl.Manifests, 0700); err != nil {
			return
		}
	}

	if len(component.DataInjections) > 0 {
		cl.DataInjections = filepath.Join(base, "data-injections")
		if err = utils.CreateDirectory(cl.DataInjections, 0700); err != nil {
			return
		}
	}

	if c.Dirs == nil {
		c.Dirs = make(map[string]*ComponentPaths)
	}

	c.Dirs[name] = cl
	return cl, nil
}
