// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package testutil

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"testing"

	goyaml "github.com/goccy/go-yaml"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
)

// ChecksumZarfYAMLContent returns sha256(marshal(pkg)) with pkg.Build zeroed.
// Build carries intrinsically non-reproducible values (timestamp, user,
// hostname, CLI version) so it is excluded; the result pins everything else
// and must be stable across build hosts.
func ChecksumZarfYAMLContent(t *testing.T, pkg v1alpha1.ZarfPackage) string {
	t.Helper()
	pkg.Build = v1alpha1.ZarfBuildData{}
	b, err := goyaml.Marshal(pkg)
	require.NoError(t, err)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// RequireNoBackslashInPackagePaths asserts every path field serialized into a
// built zarf.yaml uses forward-slash separators, so artifacts are byte-identical
// across build hosts.
func RequireNoBackslashInPackagePaths(t *testing.T, pkg v1alpha1.ZarfPackage) {
	t.Helper()

	check := func(field, value string) {
		require.Falsef(t, strings.ContainsRune(value, '\\'), "%s contains a backslash: %q", field, value)
	}

	for i, f := range pkg.Values.Files {
		check("values.files["+strconv.Itoa(i)+"]", f)
	}

	for ci, comp := range pkg.Components {
		prefix := "components[" + strconv.Itoa(ci) + "]"
		for fi, f := range comp.Files {
			check(prefix+".files["+strconv.Itoa(fi)+"].source", f.Source)
		}
		for chi, ch := range comp.Charts {
			check(prefix+".charts["+strconv.Itoa(chi)+"].localPath", ch.LocalPath)
			for vi, v := range ch.ValuesFiles {
				check(prefix+".charts["+strconv.Itoa(chi)+"].valuesFiles["+strconv.Itoa(vi)+"]", v)
			}
		}
		for mi, m := range comp.Manifests {
			for fi, f := range m.Files {
				check(prefix+".manifests["+strconv.Itoa(mi)+"].files["+strconv.Itoa(fi)+"]", f)
			}
			for ki, k := range m.Kustomizations {
				check(prefix+".manifests["+strconv.Itoa(mi)+"].kustomizations["+strconv.Itoa(ki)+"]", k)
			}
		}
		for di, d := range comp.DataInjections {
			check(prefix+".dataInjections["+strconv.Itoa(di)+"].source", d.Source)
		}
		actionsByName := map[string][]v1alpha1.ZarfComponentAction{
			"before":    comp.Actions.OnCreate.Before,
			"after":     comp.Actions.OnCreate.After,
			"onFailure": comp.Actions.OnCreate.OnFailure,
			"onSuccess": comp.Actions.OnCreate.OnSuccess,
		}
		for name, actions := range actionsByName {
			for ai, action := range actions {
				if action.Dir != nil {
					check(prefix+".actions.onCreate."+name+"["+strconv.Itoa(ai)+"].dir", *action.Dir)
				}
			}
		}
		check(prefix+".actions.onCreate.defaults.dir", comp.Actions.OnCreate.Defaults.Dir)
	}
}
