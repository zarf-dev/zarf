// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/internal/template"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/value"
)

// validateTemplateRefs ensures every go-templated .Values reference in the package's components can
// be resolved before any component is deployed. A reference is satisfiable if it resolves against the
// already-known values, or if any setValues action anywhere in the package declares it. This turns
// the late, mid-deploy missingkey errors from template.Apply into a single up-front failure so a
// package never half-deploys.
func validateTemplateRefs(ctx context.Context, pkgLayout *layout.PackageLayout, vals value.Values) error {
	if pkgLayout == nil {
		return fmt.Errorf("pkg layout is required")
	}
	components := pkgLayout.Pkg.Components
	defined := buildDefinedValues(components, vals)

	var errs []error
	for _, component := range components {
		for _, action := range onDeployActions(component) {
			if !action.ShouldTemplate() {
				continue
			}
			location := fmt.Sprintf("component %q action %q", component.Name, actionLabel(action))
			for _, s := range actionTemplateStrings(action) {
				errs = append(errs, checkTemplateString(s, defined, location)...)
			}
		}

		sources, err := componentFileSources(ctx, pkgLayout, component)
		if err != nil {
			return err
		}
		for _, src := range sources {
			errs = append(errs, checkTemplateString(src.content, defined, src.location)...)
		}
	}
	return errors.Join(errs...)
}

// definedValues describes the value keys a package can provide at deploy time.
type definedValues struct {
	vals         value.Values
	setValueKeys [][]string
	setValueRoot bool
}

func buildDefinedValues(components []v1alpha1.ZarfComponent, vals value.Values) definedValues {
	d := definedValues{vals: vals}
	if d.vals == nil {
		d.vals = value.Values{}
	}
	for _, component := range components {
		for _, action := range onDeployActions(component) {
			for _, sv := range action.SetValues {
				if sv.Key == "." {
					d.setValueRoot = true
					continue
				}
				if segments := pathSegments(sv.Key); len(segments) > 0 {
					d.setValueKeys = append(d.setValueKeys, segments)
				}
			}
		}
	}
	return d
}

func (d definedValues) hasValue(path []string) bool {
	if d.setValueRoot {
		return true
	}
	for _, key := range d.setValueKeys {
		if hasPrefix(path, key) {
			return true
		}
	}
	p := value.Path("." + strings.Join(path, "."))
	_, err := d.vals.Extract(p)
	return err == nil
}

func checkTemplateString(s string, defined definedValues, location string) []error {
	refs, err := template.ReferencedKeys(s)
	if err != nil {
		return []error{fmt.Errorf("%s: invalid go-template: %w", location, err)}
	}
	var errs []error
	for _, path := range refs.Values {
		if !defined.hasValue(path) {
			errs = append(errs, fmt.Errorf("%s: references undefined value .Values.%s", location, strings.Join(path, ".")))
		}
	}
	return errs
}

// templateSource pairs a template string with a human-readable location for error messages.
type templateSource struct {
	content  string
	location string
}

// componentFileSources extracts and reads the go-templated manifest and file contents for a
// component. Components without templated manifests or files require no extraction.
func componentFileSources(ctx context.Context, pkgLayout *layout.PackageLayout, component v1alpha1.ZarfComponent) (_ []templateSource, err error) {
	hasManifests := false
	for _, m := range component.Manifests {
		if m.IsTemplate() {
			hasManifests = true
			break
		}
	}
	hasFiles := false
	for _, f := range component.Files {
		if f.IsTemplate() {
			hasFiles = true
			break
		}
	}
	if !hasManifests && !hasFiles {
		return nil, nil
	}

	tmpDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, os.RemoveAll(tmpDir))
	}()

	var sources []templateSource
	if hasManifests {
		manifestDir, err := pkgLayout.GetComponentDir(ctx, tmpDir, component.Name, layout.ManifestsComponentDir)
		if err != nil {
			return nil, err
		}
		for _, manifest := range component.Manifests {
			if !manifest.IsTemplate() {
				continue
			}
			for idx := range manifest.Files {
				path := filepath.Join(manifestDir, fmt.Sprintf("%s-%d.yaml", manifest.Name, idx))
				content, err := os.ReadFile(path)
				if err != nil {
					return nil, err
				}
				sources = append(sources, templateSource{
					content:  string(content),
					location: fmt.Sprintf("component %q manifest %q", component.Name, manifest.Name),
				})
			}
		}
	}
	if hasFiles {
		filesDir, err := pkgLayout.GetComponentDir(ctx, tmpDir, component.Name, layout.FilesComponentDir)
		if err != nil {
			return nil, err
		}
		for fileIdx, file := range component.Files {
			if !file.IsTemplate() {
				continue
			}
			fileLocation := filepath.Join(filesDir, strconv.Itoa(fileIdx), filepath.Base(file.Target))
			if helpers.InvalidPath(fileLocation) {
				fileLocation = filepath.Join(filesDir, strconv.Itoa(fileIdx))
			}
			fileList := []string{fileLocation}
			if helpers.IsDir(fileLocation) {
				fileList, err = helpers.RecursiveFileList(fileLocation, nil, false)
				if err != nil {
					return nil, err
				}
			}
			for _, subFile := range fileList {
				isText, err := helpers.IsTextFile(subFile)
				if err != nil {
					return nil, err
				}
				if !isText {
					continue
				}
				content, err := os.ReadFile(subFile)
				if err != nil {
					return nil, err
				}
				sources = append(sources, templateSource{
					content:  string(content),
					location: fmt.Sprintf("component %q file %q", component.Name, file.Target),
				})
			}
		}
	}
	return sources, nil
}

func onDeployActions(c v1alpha1.ZarfComponent) []v1alpha1.ZarfComponentAction {
	s := c.Actions.OnDeploy
	actions := make([]v1alpha1.ZarfComponentAction, 0, len(s.Before)+len(s.After)+len(s.OnSuccess)+len(s.OnFailure))
	actions = append(actions, s.Before...)
	actions = append(actions, s.After...)
	actions = append(actions, s.OnSuccess...)
	actions = append(actions, s.OnFailure...)
	return actions
}

func actionTemplateStrings(a v1alpha1.ZarfComponentAction) []string {
	if a.Wait != nil {
		var out []string
		if c := a.Wait.Cluster; c != nil {
			out = append(out, c.Kind, c.Name, c.Namespace, c.Condition)
		}
		if n := a.Wait.Network; n != nil {
			out = append(out, n.Protocol, n.Address)
		}
		return out
	}
	return []string{a.Cmd}
}

func actionLabel(a v1alpha1.ZarfComponentAction) string {
	if a.Description != "" {
		return a.Description
	}
	if a.Wait != nil {
		return "wait"
	}
	return helpers.Truncate(a.Cmd, 60, false)
}

// pathSegments splits a value.Path key (e.g. ".db.host") into its segments, dropping the leading dot.
func pathSegments(key string) []string {
	s := strings.TrimPrefix(key, ".")
	if s == "" {
		return nil
	}
	return strings.Split(s, ".")
}

func hasPrefix(path, prefix []string) bool {
	if len(prefix) > len(path) {
		return false
	}
	for i, seg := range prefix {
		if path[i] != seg {
			return false
		}
	}
	return true
}
