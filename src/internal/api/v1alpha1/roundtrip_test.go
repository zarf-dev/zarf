// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package v1alpha1

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	internalv1beta1 "github.com/zarf-dev/zarf/src/internal/api/v1beta1"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

// defaultFuzzIterations specifies number of fuzzing iterations, higher number
// will quickly raise the time needed to run them. The 20 iterations balances
// time (currently <60s) with coverage.
const defaultFuzzIterations = 20

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
	original.Build.SetOriginalAPIVersion(v1alpha1.APIVersion)

	roundTripped := ConvertFromGeneric(ConvertToGeneric(original))
	require.Equal(t, original, roundTripped)
}

// TestConvertGenericRoundTripFuzz reflectively populates every field of a ZarfPackage with random
// values and asserts the generic round-trip reproduces it exactly. Walking the struct by reflection
// means a newly added field is exercised automatically, so a field the conversion forgets to carry
// is caught here rather than silently dropped.
func TestConvertGenericRoundTripFuzz(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(1))
	for i := range defaultFuzzIterations {
		var pkg v1alpha1.ZarfPackage
		testutil.FillValue(reflect.ValueOf(&pkg).Elem(), rng)

		// apiVersion and kind are canonicalized on conversion, so they never round-trip an arbitrary
		// value; pin them to valid forms and let every other field vary.
		pkg.APIVersion = v1alpha1.APIVersion
		pkg.Kind = v1alpha1.ZarfPackageConfig
		pkg.Build.SetOriginalAPIVersion(v1alpha1.APIVersion)

		roundTripped := ConvertFromGeneric(ConvertToGeneric(pkg))
		require.Equalf(t, pkg, roundTripped, "round-trip diverged on iteration %d", i)
	}
}

// TestConvertV1alpha1V1beta1RoundTripFuzz verifies that fields shared by v1alpha1 and v1beta1
// survive a conversion through v1beta1. Reflection exercises newly added v1alpha1 fields by
// default; cross-version incompatibilities are deliberately excluded below.
func TestConvertV1alpha1V1beta1RoundTripFuzz(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(1))
	for i := range defaultFuzzIterations {
		var pkg v1alpha1.ZarfPackage
		testutil.FillValue(reflect.ValueOf(&pkg).Elem(), rng)
		populateValidV1alpha1ChartSources(&pkg, rng, i)

		v1beta1Pkg := internalv1beta1.ConvertFromGeneric(ConvertToGeneric(pkg))
		roundTripped := ConvertFromGeneric(internalv1beta1.ConvertToGeneric(v1beta1Pkg))
		require.Emptyf(t, cmp.Diff(pkg, roundTripped, v1alpha1V1beta1RoundTripExclusions()...), "cross-version round-trip diverged on iteration %d", i)
	}
}

func populateValidV1alpha1ChartSources(pkg *v1alpha1.ZarfPackage, rng *rand.Rand, iteration int) {
	chartIndex := iteration
	for ci := range pkg.Components {
		for chi := range pkg.Components[ci].Charts {
			chart := &pkg.Components[ci].Charts[chi]
			chart.URL, chart.RepoName, chart.GitPath, chart.LocalPath, chart.Version = "", "", "", "", ""

			switch rng.Intn(4) {
			case 0:
				chart.URL = fmt.Sprintf("https://charts%d.example.com", rng.Intn(1<<30))
				chart.RepoName = fmt.Sprintf("chart-%d", chartIndex)
				chart.Version = fmt.Sprintf("%d", rng.Intn(1<<30))
			case 1:
				chart.URL = fmt.Sprintf("https://git%d.example.com/chart-%d.git", rng.Intn(1<<30), chartIndex)
				chart.GitPath = fmt.Sprintf("charts/chart-%d", chartIndex)
				switch rng.Intn(3) {
				case 0:
					chart.Version = fmt.Sprintf("%d", rng.Intn(1<<30))
				case 1:
					chart.Version = fmt.Sprintf("%040x", rng.Uint64())
				case 2:
					chart.URL += fmt.Sprintf("@refs/heads/branch-%d", rng.Intn(1<<30))
				}
			case 2:
				chart.LocalPath = fmt.Sprintf("charts/chart-%d", chartIndex)
			case 3:
				chart.URL = fmt.Sprintf("oci://registry%d.example.com/chart-%d", rng.Intn(1<<30), chartIndex)
				if rng.Intn(2) == 0 {
					chart.Version = fmt.Sprintf("%d", rng.Intn(1<<30))
				} else {
					chart.URL += fmt.Sprintf("@sha256:%064x", rng.Uint64())
				}
			}
			chartIndex++
		}
	}
}

// v1alpha1V1beta1RoundTripExclusions lists the v1alpha1 fields that v1beta1 cannot represent.
// The fuzz test replaces chart sources with schema-valid generated values and ignores only these
// fields when comparing the result.
//
//   - fields removed from v1beta1: package.constants, package.variables, metadata.yolo,
//     build.differentialMissing, component.default, component.group, component.dataInjections,
//     component.deprecatedScripts, component.only.cluster.distros, component.import.name, and
//     chart.variables.
//   - boolean pointer presence is lost: metadata.allowNamespaceOverride is projected to the inverse
//     PreventNamespaceOverride bool; component.required to optional; chart.schemaValidation to
//     SkipSchemaValidation; and manifest.template and file.template to EnableTemplating. In each
//     case, nil is indistinguishable from one of the boolean values.
//   - package.apiVersion and package.kind are canonicalized to the target API.
//   - metadata annotations using metadata.url, metadata.image, metadata.authors,
//     metadata.documentation, metadata.source, or metadata.vendor collide with v1alpha1's
//     dedicated metadata fields during projection.
//   - originalAPIVersion is internal tracking and is set by the version that loads or creates the
//     package.
//   - component.healthChecks are projected to onDeploy/onSuccess wait actions and cannot be
//     reconstructed as health checks.
//   - actionSet.after is folded into v1beta1's actionSet.onSuccess, so both lists differ on return.
//     action.deprecatedSetVariable and action.setVariables have no v1beta1 equivalents; and an
//     action.template false pointer cannot be distinguished from nil after projection to
//     EnableTemplating. action.wait.cluster.condition defaults from empty to "exists" in v1beta1.
func v1alpha1V1beta1RoundTripExclusions() cmp.Options {
	return cmp.Options{
		cmpopts.IgnoreFields(v1alpha1.ZarfPackage{}, "APIVersion", "Kind", "Constants", "Variables"),
		cmpopts.IgnoreFields(v1alpha1.ZarfMetadata{}, "YOLO", "AllowNamespaceOverride"),
		cmpopts.IgnoreMapEntries(func(key, _ string) bool {
			switch key {
			case "metadata.url", "metadata.image", "metadata.authors", "metadata.documentation", "metadata.source", "metadata.vendor":
				return true
			default:
				return false
			}
		}),
		cmpopts.IgnoreFields(v1alpha1.ZarfBuildData{}, "DifferentialMissing"),
		cmpopts.IgnoreUnexported(v1alpha1.ZarfBuildData{}),
		cmpopts.IgnoreFields(v1alpha1.ZarfComponent{}, "Default", "Required", "DeprecatedGroup", "DataInjections", "DeprecatedScripts", "HealthChecks"),
		cmpopts.IgnoreFields(v1alpha1.ZarfComponentOnlyCluster{}, "Distros"),
		cmpopts.IgnoreFields(v1alpha1.ZarfComponentImport{}, "Name"),
		cmpopts.IgnoreFields(v1alpha1.ZarfComponentActionSet{}, "After", "OnSuccess"),
		cmpopts.IgnoreFields(v1alpha1.ZarfComponentAction{}, "DeprecatedSetVariable", "SetVariables", "Template"),
		cmpopts.IgnoreFields(v1alpha1.ZarfComponentActionWaitCluster{}, "Condition"),
		cmpopts.IgnoreFields(v1alpha1.ZarfChart{}, "Variables", "SchemaValidation"),
		cmpopts.IgnoreFields(v1alpha1.ZarfManifest{}, "Template"),
		cmpopts.IgnoreFields(v1alpha1.ZarfFile{}, "Template"),
	}
}
