// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package v1alpha1

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
)

// TestConvertGenericRoundTripLossless asserts that decoding a v1alpha1 package, converting it to
// the generic representation and back, reproduces the original exactly. layout and zoci load built
// v1alpha1 packages through this round-trip, so any drift would change packages across build hosts
func TestConvertGenericRoundTripLossless(t *testing.T) {
	t.Parallel()

	b := func(v bool) *bool { return &v }
	i := func(v int) *int { return &v }
	s := func(v string) *string { return &v }

	original := v1alpha1.ZarfPackage{
		APIVersion: v1alpha1.APIVersion,
		Kind:       v1alpha1.ZarfInitConfig,
		Metadata: v1alpha1.ZarfMetadata{
			Name:                   "round-trip",
			Description:            "desc",
			Version:                "1.2.3",
			URL:                    "https://example.com",
			Image:                  "https://example.com/img.png",
			Uncompressed:           true,
			Architecture:           "arm64",
			YOLO:                   true,
			Authors:                "author",
			Documentation:          "https://docs.example.com",
			Source:                 "https://github.com/example",
			Vendor:                 "vendor",
			AggregateChecksum:      "checksum",
			AllowNamespaceOverride: b(false),
			Annotations:            map[string]string{"k": "v"},
		},
		Build: v1alpha1.ZarfBuildData{
			Terminal:                   "host",
			User:                       "user",
			Architecture:               "arm64",
			Timestamp:                  "Mon, 02 Jan 2006 15:04:05 -0700",
			Version:                    "v0.30.0",
			Migrations:                 []string{"scripts-to-actions", "pluralize-set-variable"},
			RegistryOverrides:          map[string]string{"reg": "override"},
			Differential:               true,
			DifferentialPackageVersion: "1.2.2",
			DifferentialMissing:        []string{"comp-x"},
			Flavor:                     "prod",
			Signed:                     b(true),
			VersionRequirements:        []v1alpha1.VersionRequirement{{Version: ">=1.0.0", Reason: "needs feature"}},
			ProvenanceFiles:            []string{"prov.json"},
		},
		Components: []v1alpha1.ZarfComponent{
			{
				Name:            "comp",
				Description:     "comp desc",
				Default:         true,
				Required:        b(true),
				DeprecatedGroup: "group",
				Only: v1alpha1.ZarfComponentOnlyTarget{
					LocalOS: "linux",
					Cluster: v1alpha1.ZarfComponentOnlyCluster{Architecture: "arm64", Distros: []string{"k3s"}},
					Flavor:  "prod",
				},
				Import: v1alpha1.ZarfComponentImport{Name: "imp", Path: "path", URL: "oci://example.com/pkg"},
				Repos:  []string{"https://github.com/example/repo"},
				Images: []string{"nginx:latest"},
				ImageArchives: []v1alpha1.ImageArchive{
					{Path: "images.tar", Images: []string{"busybox:1.36"}},
				},
				StateAccess: []v1alpha1.StateAccessKey{v1alpha1.StateAccessRegistryCredentials},
				Charts: []v1alpha1.ZarfChart{
					{
						Name:                 "chart",
						Version:              "1.0.0",
						URL:                  "https://charts.example.com",
						RepoName:             "chart",
						Namespace:            "default",
						ReleaseName:          "rel",
						NoWait:               true,
						ValuesFiles:          []string{"values.yaml"},
						TemplatedValuesFiles: []string{"templated.yaml"},
						SchemaValidation:     b(false),
						ServerSideApply:      "auto",
						Variables:            []v1alpha1.ZarfChartVariable{{Name: "VAR", Description: "d", Path: "p"}},
						Values:               []v1alpha1.ZarfChartValue{{SourcePath: ".a", TargetPath: ".b"}},
					},
				},
				Manifests: []v1alpha1.ZarfManifest{
					{
						Name:                       "manifest",
						Namespace:                  "default",
						Files:                      []string{"deploy.yaml"},
						Kustomizations:             []string{"kustomize"},
						KustomizeAllowAnyDirectory: true,
						NoWait:                     true,
						ServerSideApply:            "true",
					},
				},
				Files: []v1alpha1.ZarfFile{
					{Source: "src", Shasum: "sha", Target: "tgt", Executable: true, Symlinks: []string{"ln"}, ExtractPath: "extract", Template: b(true)},
				},
				DataInjections: []v1alpha1.ZarfDataInjection{
					{Source: "src", Target: v1alpha1.ZarfContainerTarget{Namespace: "ns", Selector: "app=x", Container: "c", Path: "/p"}, Compress: true},
				},
				Actions: v1alpha1.ZarfComponentActions{
					OnDeploy: v1alpha1.ZarfComponentActionSet{
						Defaults: v1alpha1.ZarfComponentActionDefaults{
							Mute:            true,
							MaxTotalSeconds: 60,
							MaxRetries:      2,
							Dir:             "dir",
							Env:             []string{"K=V"},
							Shell:           v1alpha1.Shell{Windows: "pwsh", Linux: "sh", Darwin: "zsh"},
						},
						Before:    []v1alpha1.ZarfComponentAction{{Cmd: "before", Mute: b(true), MaxTotalSeconds: i(30), MaxRetries: i(3), Dir: s("d")}},
						After:     []v1alpha1.ZarfComponentAction{{Cmd: "after"}},
						OnSuccess: []v1alpha1.ZarfComponentAction{{Cmd: "success"}},
						OnFailure: []v1alpha1.ZarfComponentAction{{Cmd: "failure"}},
					},
				},
				HealthChecks: []v1alpha1.NamespacedObjectKindReference{
					{APIVersion: "v1", Kind: "Pod", Namespace: "ns", Name: "n"},
				},
			},
		},
		Constants: []v1alpha1.Constant{{Name: "CONST", Value: "val", Description: "d", Pattern: ".*"}},
		Variables: []v1alpha1.InteractiveVariable{
			{Variable: v1alpha1.Variable{Name: "VAR", Sensitive: true, Type: "raw"}, Description: "d", Default: "def", Prompt: true},
		},
		Values:        v1alpha1.ZarfValues{Files: []string{"vals.yaml"}, Schema: "schema.json"},
		Documentation: map[string]string{"doc": "doc.md"},
	}

	roundTripped := ConvertFromGeneric(ConvertToGeneric(original))
	require.Equal(t, original, roundTripped)
}

// TestConvertGenericRoundTripPreservesRequired asserts that a component's Required pointer survives
// the generic round-trip byte-for-byte, including an unset (nil) value. v1alpha1 defaults an unset
// required to optional, so collapsing nil to &false would drift the serialized package. An empty
// apiVersion is the implicit v1alpha1 form and must round-trip identically to the explicit one.
func TestConvertGenericRoundTripPreservesRequired(t *testing.T) {
	t.Parallel()

	b := func(v bool) *bool { return &v }
	for _, apiVersion := range []string{v1alpha1.APIVersion, ""} {
		for _, required := range []*bool{nil, b(false), b(true)} {
			original := v1alpha1.ZarfPackage{
				APIVersion: apiVersion,
				Kind:       v1alpha1.ZarfPackageConfig,
				Components: []v1alpha1.ZarfComponent{{Name: "comp", Required: required}},
			}
			roundTripped := ConvertFromGeneric(ConvertToGeneric(original))
			require.Equal(t, required, roundTripped.Components[0].Required)
		}
	}
}

// lossyFields lists v1alpha1 fields that intentionally do not survive the generic round-trip, keyed
// by struct type name then field name. The fuzz filler leaves them zero so they compare equal. A
// field NOT listed here is expected to round-trip; adding one to the API without either carrying it
// through the conversion or justifying it here will fail TestConvertGenericRoundTripFuzz on purpose.
var lossyFields = map[string]map[string]string{
	"ZarfComponent": {
		"DeprecatedScripts": "deprecated pre-actions format; not carried through the conversion",
	},
	"ZarfFile": {
		"Template": "*bool collapses into the generic EnableTemplating bool, so nil and false merge",
	},
	"ZarfComponentAction": {
		"Template": "*bool collapses into the generic EnableTemplating bool, so nil and false merge",
	},
	"ZarfChartValue": {
		"ExcludePaths": "no equivalent in the generic/v1beta1 model; dropped by the conversion",
	},
}

// TestConvertGenericRoundTripFuzz reflectively populates every field of a ZarfPackage with random,
// non-zero values and asserts the generic round-trip reproduces it exactly. Walking the struct by
// reflection means a newly added field is exercised automatically, so a field the conversion forgets
// to carry is caught here rather than silently dropped. Fields known not to round-trip are recorded
// in lossyFields and left at their zero value.
func TestConvertGenericRoundTripFuzz(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(1))
	for i := range 1000 {
		var pkg v1alpha1.ZarfPackage
		fillValue(reflect.ValueOf(&pkg).Elem(), rng)

		// apiVersion and kind are canonicalized on conversion, so they never round-trip an arbitrary
		// value; pin them to valid forms and let every other field vary.
		pkg.APIVersion = v1alpha1.APIVersion
		pkg.Kind = v1alpha1.ZarfPackageConfig

		roundTripped := ConvertFromGeneric(ConvertToGeneric(pkg))
		require.Equalf(t, pkg, roundTripped, "round-trip diverged on iteration %d", i)
	}
}

// fillValue recursively sets v to a random, non-zero value. Unexported fields cannot be set via
// reflection and are left zero; the conversion never populates them, so they stay equal on both
// sides. Fields recorded in lossyFields are also left zero.
func fillValue(v reflect.Value, rng *rand.Rand) {
	switch v.Kind() {
	case reflect.Pointer:
		v.Set(reflect.New(v.Type().Elem()))
		fillValue(v.Elem(), rng)
	case reflect.Struct:
		lossy := lossyFields[v.Type().Name()]
		for i := range v.NumField() {
			f := v.Field(i)
			if _, skip := lossy[v.Type().Field(i).Name]; skip || !f.CanSet() {
				continue
			}
			fillValue(f, rng)
		}
	case reflect.Slice:
		n := 1 + rng.Intn(2)
		s := reflect.MakeSlice(v.Type(), n, n)
		for i := range n {
			fillValue(s.Index(i), rng)
		}
		v.Set(s)
	case reflect.Map:
		m := reflect.MakeMap(v.Type())
		for range 1 + rng.Intn(2) {
			key := reflect.New(v.Type().Key()).Elem()
			fillValue(key, rng)
			val := reflect.New(v.Type().Elem()).Elem()
			fillValue(val, rng)
			m.SetMapIndex(key, val)
		}
		v.Set(m)
	case reflect.String:
		v.SetString(fmt.Sprintf("s%d", rng.Intn(1<<30)))
	case reflect.Bool:
		v.SetBool(rng.Intn(2) == 1)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(int64(1 + rng.Intn(1000)))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(uint64(1 + rng.Intn(1000)))
	case reflect.Float32, reflect.Float64:
		v.SetFloat(float64(1 + rng.Intn(1000)))
	}
}
