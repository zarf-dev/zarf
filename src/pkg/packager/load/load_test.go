// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package load

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	goyaml "github.com/goccy/go-yaml"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/convert"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/pkg/feature"
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestPackageDefinitionV1Beta1ImageIDs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		imported []v1beta1.Image
		middle   []v1beta1.Image
		head     []v1beta1.Image
		want     []v1beta1.Image
	}{
		{
			name:     "matching ID replaces reference and overrides source",
			imported: []v1beta1.Image{{ID: "registry", Name: "registry:3", Source: "daemon"}},
			head:     []v1beta1.Image{{ID: "registry", Name: "enterprise.example.com/registry:3", Source: "registry"}},
			want:     []v1beta1.Image{{ID: "registry", Name: "enterprise.example.com/registry:3", Source: "registry"}},
		},
		{
			name:     "matching ID inherits omitted source",
			imported: []v1beta1.Image{{ID: "registry", Name: "registry:3", Source: "daemon"}},
			head:     []v1beta1.Image{{ID: "registry", Name: "enterprise.example.com/registry:3"}},
			want:     []v1beta1.Image{{ID: "registry", Name: "enterprise.example.com/registry:3", Source: "daemon"}},
		},
		{
			name:     "matching name inherits ID for later replacement",
			imported: []v1beta1.Image{{ID: "registry", Name: "registry:3"}},
			middle:   []v1beta1.Image{{Name: "registry:3", Source: "daemon"}},
			head:     []v1beta1.Image{{ID: "registry", Name: "enterprise.example.com/registry:3"}},
			want:     []v1beta1.Image{{ID: "registry", Name: "enterprise.example.com/registry:3", Source: "daemon"}},
		},
		{
			name:     "matching name adds ID for later replacement",
			imported: []v1beta1.Image{{Name: "registry:3", Source: "daemon"}},
			middle:   []v1beta1.Image{{ID: "registry", Name: "registry:3"}},
			head:     []v1beta1.Image{{ID: "registry", Name: "enterprise.example.com/registry:3"}},
			want:     []v1beta1.Image{{ID: "registry", Name: "enterprise.example.com/registry:3", Source: "daemon"}},
		},
		{
			name:     "matching name overrides different ID",
			imported: []v1beta1.Image{{ID: "old", Name: "registry:3"}},
			head:     []v1beta1.Image{{ID: "registry", Name: "registry:3"}},
			want:     []v1beta1.Image{{ID: "registry", Name: "registry:3"}},
		},
		{
			name:     "empty IDs do not match and new images append",
			imported: []v1beta1.Image{{Name: "nginx:1.27"}, {ID: "registry", Name: "registry:3"}},
			head:     []v1beta1.Image{{Name: "redis:7"}, {ID: "app", Name: "example.com/app:1"}},
			want:     []v1beta1.Image{{Name: "nginx:1.27"}, {ID: "registry", Name: "registry:3"}, {Name: "redis:7"}, {ID: "app", Name: "example.com/app:1"}},
		},
		{
			name:     "ID match takes precedence over earlier name match",
			imported: []v1beta1.Image{{ID: "other", Name: "enterprise.example.com/registry:3"}, {ID: "registry", Name: "registry:3"}},
			head:     []v1beta1.Image{{ID: "registry", Name: "enterprise.example.com/registry:3"}},
			want:     []v1beta1.Image{{ID: "other", Name: "enterprise.example.com/registry:3"}, {ID: "registry", Name: "enterprise.example.com/registry:3"}},
		},
		{
			name:     "nearest importing component wins in nested imports",
			imported: []v1beta1.Image{{ID: "registry", Name: "registry:3"}},
			middle:   []v1beta1.Image{{ID: "registry", Name: "mirror.example.com/registry:3"}},
			head:     []v1beta1.Image{{ID: "registry", Name: "enterprise.example.com/registry:3"}},
			want:     []v1beta1.Image{{ID: "registry", Name: "enterprise.example.com/registry:3"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeYAML := func(name string, value any) {
				t.Helper()
				data, err := goyaml.MarshalWithOptions(value, goyaml.UseJSONMarshaler())
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(filepath.Join(dir, name), data, 0o600))
			}
			writeYAML("base.yaml", v1beta1.ComponentConfig{
				APIVersion: v1beta1.APIVersion,
				Kind:       v1beta1.ZarfComponentConfig,
				Metadata:   v1beta1.ComponentMetadata{Name: "base"},
				Component:  v1beta1.ComponentSpec{Images: tt.imported},
			})
			writeYAML("middle.yaml", v1beta1.ComponentConfig{
				APIVersion: v1beta1.APIVersion,
				Kind:       v1beta1.ZarfComponentConfig,
				Metadata:   v1beta1.ComponentMetadata{Name: "middle"},
				Component: v1beta1.ComponentSpec{
					Import: v1beta1.ComponentImport{Local: []v1beta1.ComponentImportLocal{{Path: "base.yaml"}}},
					Images: tt.middle,
				},
			})
			writeYAML(layout.ZarfYAML, v1beta1.Package{
				APIVersion: v1beta1.APIVersion,
				Kind:       v1beta1.ZarfPackageConfig,
				Metadata:   v1beta1.PackageMetadata{Name: "image-ids"},
				Components: []v1beta1.Component{{
					Name: "app",
					ComponentSpec: v1beta1.ComponentSpec{
						Import: v1beta1.ComponentImport{Local: []v1beta1.ComponentImportLocal{{Path: "middle.yaml"}}},
						Images: tt.head,
					},
				}},
			})

			pkg, err := PackageDefinition(testutil.TestContext(t), dir, DefinitionOptions{})
			require.NoError(t, err)
			require.Len(t, pkg.Components, 1)
			require.Equal(t, tt.want, convert.PackageToV1beta1(pkg).Components[0].Images)
		})
	}
}

func TestV1Beta1ComponentImageIDs(t *testing.T) {
	t.Parallel()

	regular := func(id, name string) v1beta1.Image { return v1beta1.Image{ID: id, Name: name} }
	archived := func(path, id, name string) v1beta1.ImageArchive {
		return v1beta1.ImageArchive{Path: path, Images: []v1beta1.ArchiveImage{{ID: id, Name: name}}}
	}
	for _, tc := range []struct {
		name     string
		imported v1beta1.ComponentSpec
		head     v1beta1.ComponentSpec
		wantErr  bool
	}{
		{
			name:     "regular image overrides imported regular image",
			imported: v1beta1.ComponentSpec{Images: []v1beta1.Image{regular("app", "example.com/app:1")}},
			head:     v1beta1.ComponentSpec{Images: []v1beta1.Image{regular("app", "example.com/app:2")}},
		},
		{
			name:     "archive image is available by ID after import",
			imported: v1beta1.ComponentSpec{ImageArchives: []v1beta1.ImageArchive{archived("images.tar", "app", "app.internal/app:1")}},
		},
		{
			name:     "regular image cannot override imported archive image",
			imported: v1beta1.ComponentSpec{ImageArchives: []v1beta1.ImageArchive{archived("images.tar", "app", "app.internal/app:1")}},
			head:     v1beta1.ComponentSpec{Images: []v1beta1.Image{regular("app", "example.com/app:2")}},
			wantErr:  true,
		},
		{
			name:     "archive image cannot override imported regular image",
			imported: v1beta1.ComponentSpec{Images: []v1beta1.Image{regular("app", "example.com/app:1")}},
			head:     v1beta1.ComponentSpec{ImageArchives: []v1beta1.ImageArchive{archived("images.tar", "app", "app.internal/app:2")}},
			wantErr:  true,
		},
		{
			name:     "archive images cannot override imported archive images",
			imported: v1beta1.ComponentSpec{ImageArchives: []v1beta1.ImageArchive{archived("images.tar", "app", "app.internal/app:1")}},
			head:     v1beta1.ComponentSpec{ImageArchives: []v1beta1.ImageArchive{archived("replacement.tar", "app", "app.internal/app:2")}},
			wantErr:  true,
		},
		{
			name:     "duplicate authored regular IDs cannot merge away",
			imported: v1beta1.ComponentSpec{Images: []v1beta1.Image{regular("app", "example.com/app:1")}},
			head:     v1beta1.ComponentSpec{Images: []v1beta1.Image{regular("app", "example.com/app:2"), regular("app", "example.com/app:3")}},
			wantErr:  true,
		},
		{
			name:     "duplicate imported regular IDs cannot be overridden away",
			imported: v1beta1.ComponentSpec{Images: []v1beta1.Image{regular("app", "example.com/app:1"), regular("app", "example.com/app:2")}},
			head:     v1beta1.ComponentSpec{Images: []v1beta1.Image{regular("app", "example.com/app:3")}},
			wantErr:  true,
		},
		{
			name: "duplicate IDs within an archive",
			head: v1beta1.ComponentSpec{ImageArchives: []v1beta1.ImageArchive{{Path: "images.tar", Images: []v1beta1.ArchiveImage{
				{ID: "app", Name: "app.internal/app:1"}, {ID: "app", Name: "app.internal/app:2"},
			}}}},
			wantErr: true,
		},
		{
			name:    "duplicate IDs across authored archives",
			head:    v1beta1.ComponentSpec{ImageArchives: []v1beta1.ImageArchive{archived("one.tar", "app", "app.internal/app:1"), archived("two.tar", "app", "app.internal/app:2")}},
			wantErr: true,
		},
		{
			name: "duplicate IDs across authored regular and archive images",
			head: v1beta1.ComponentSpec{
				Images:        []v1beta1.Image{regular("app", "example.com/app:1")},
				ImageArchives: []v1beta1.ImageArchive{archived("images.tar", "app", "app.internal/app:2")},
			},
			wantErr: true,
		},
		{
			name:     "empty IDs are allowed across all image sources",
			imported: v1beta1.ComponentSpec{Images: []v1beta1.Image{regular("", "example.com/app:1")}},
			head:     v1beta1.ComponentSpec{ImageArchives: []v1beta1.ImageArchive{archived("one.tar", "", "app.internal/app:1"), archived("two.tar", "", "app.internal/app:2")}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			write := func(name string, definition any) string {
				t.Helper()
				b, err := goyaml.Marshal(definition)
				require.NoError(t, err)
				path := filepath.Join(dir, name)
				require.NoError(t, os.WriteFile(path, b, 0o600))
				return path
			}
			write("base.yaml", v1beta1.ComponentConfig{
				APIVersion: v1beta1.APIVersion, Kind: v1beta1.ZarfComponentConfig,
				Metadata: v1beta1.ComponentMetadata{Name: "base"}, Component: tc.imported,
			})
			head := tc.head
			head.Import.Local = []v1beta1.ComponentImportLocal{{Path: "base.yaml"}}
			componentPath := write("component.yaml", v1beta1.ComponentConfig{
				APIVersion: v1beta1.APIVersion, Kind: v1beta1.ZarfComponentConfig,
				Metadata: v1beta1.ComponentMetadata{Name: "app"}, Component: head,
			})
			packagePath := write("zarf.yaml", v1beta1.Package{
				APIVersion: v1beta1.APIVersion, Kind: v1beta1.ZarfPackageConfig,
				Metadata: v1beta1.PackageMetadata{Name: "image-ids"},
				Components: []v1beta1.Component{
					{Name: "app", ComponentSpec: head},
					// IDs are scoped to a component, not to the package.
					{Name: "other", ComponentSpec: v1beta1.ComponentSpec{Images: []v1beta1.Image{regular("app", "example.com/other:1")}}},
				},
			})
			ctx := testutil.TestContext(t)
			pkg, packageErr := PackageDefinition(ctx, packagePath, DefinitionOptions{})
			component, componentErr := ComponentDefinition(ctx, componentPath, ComponentOptions{})
			if tc.wantErr {
				for _, err := range []error{packageErr, componentErr} {
					require.ErrorContains(t, err, `image ID "app" is not unique`)
				}
				return
			}
			require.NoError(t, packageErr)
			require.NoError(t, componentErr)
			resolved := convert.PackageToV1beta1(pkg).Components[0]
			require.Equal(t, component.Component, resolved.ComponentSpec)
			if tc.name == "regular image overrides imported regular image" {
				require.Equal(t, tc.head.Images, resolved.Images)
			}
			if tc.name == "archive image is available by ID after import" {
				image, err := resolved.GetImageByID("app")
				require.NoError(t, err)
				require.Equal(t, "app.internal/app:1", image.Reference)
			}
		})
	}
}

func TestLoadPackageWithFlavors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		flavor      string
		expectedErr string
	}{
		{
			name:        "when all components have a flavor, inputting no flavor should error",
			flavor:      "",
			expectedErr: fmt.Sprintf("package validation failed: %s", "package does not contain any compatible components"),
		},
		{
			name:   "flavors work",
			flavor: "cashew",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			opts := DefinitionOptions{
				Flavor: tt.flavor,
			}
			_, err := PackageDefinition(context.Background(), filepath.Join("testdata", "package-with-flavors"), opts)
			if tt.expectedErr != "" {
				require.ErrorContains(t, err, tt.expectedErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestValidateV1Beta1_FormatsValidationErrors(t *testing.T) {
	t.Parallel()

	err := validateV1Beta1(context.Background(), v1beta1.Package{}, "", "")

	require.EqualError(t, err, "package validation failed:\npackage does not contain any compatible components")
}

func TestPackageDefinitionV1Beta1InitServices(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		kind     string
		service  string
		imported bool
		wantErr  string
	}{
		{name: "init with service", kind: "ZarfInitConfig", service: "registry"},
		{name: "init without service", kind: "ZarfInitConfig"},
		{name: "ordinary package with service", kind: "ZarfPackageConfig", service: "registry", wantErr: "only ZarfInitConfig packages may declare services"},
		{name: "init with imported service", kind: "ZarfInitConfig", imported: true},
		{name: "init overrides imported service", kind: "ZarfInitConfig", imported: true, service: "agent"},
		{name: "ordinary package with imported service", kind: "ZarfPackageConfig", imported: true, wantErr: "only ZarfInitConfig packages may declare services"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			component := "  - name: registry\n"
			if tt.imported {
				imported := `apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: registry
component:
  service:
    name: registry
`
				require.NoError(t, os.WriteFile(filepath.Join(dir, "registry.yaml"), []byte(imported), 0o600))
				component += "    import:\n      local:\n        - path: registry.yaml\n"
			}
			if tt.service != "" {
				component += "    service:\n      name: " + tt.service + "\n"
			}
			definition := "apiVersion: zarf.dev/v1beta1\nkind: " + tt.kind + "\nmetadata:\n  name: init-test\ncomponents:\n" + component
			require.NoError(t, os.WriteFile(filepath.Join(dir, layout.ZarfYAML), []byte(definition), 0o600))

			pkg, err := PackageDefinition(testutil.TestContext(t), dir, DefinitionOptions{})
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.kind, string(pkg.Kind))
			require.Len(t, pkg.Components, 1)
			if tt.service != "" {
				require.Equal(t, tt.service, string(pkg.Components[0].Service.Name))
			} else if tt.imported {
				require.Equal(t, "registry", string(pkg.Components[0].Service.Name))
			} else {
				require.Empty(t, pkg.Components[0].Service)
			}
		})
	}
}

func TestPackageDefinitionV1Beta1ImageSources(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		components  string
		expectedErr string
	}{
		{
			name: "conflicting sources across components with equivalent references",
			components: `  - name: first
    images:
      - name: nginx:1.27
        source: daemon
  - name: second
    images:
      - name: docker.io/library/nginx:1.27
        source: registry
`,
			expectedErr: `image "docker.io/library/nginx:1.27" has conflicting sources "daemon" and "registry"`,
		},
		{
			name: "omitted source defaults to registry",
			components: `  - name: component
    images:
      - name: nginx:1.27
      - name: nginx:1.27
        source: daemon
`,
			expectedErr: `image "docker.io/library/nginx:1.27" has conflicting sources "registry" and "daemon"`,
		},
		{
			name: "duplicate images with the same source are valid",
			components: `  - name: first
    images:
      - name: nginx:1.27
  - name: second
    images:
      - name: docker.io/library/nginx:1.27
        source: registry
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			definition := "apiVersion: zarf.dev/v1beta1\nkind: ZarfPackageConfig\nmetadata:\n  name: image-sources\ncomponents:\n" + tt.components
			require.NoError(t, os.WriteFile(filepath.Join(dir, layout.ZarfYAML), []byte(definition), 0o600))

			_, err := PackageDefinition(testutil.TestContext(t), dir, DefinitionOptions{})
			if tt.expectedErr != "" {
				require.ErrorContains(t, err, tt.expectedErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestPackageDefinitionRejectsV1Beta1ChartPathComponents(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	packageYAML := `apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: path-components
components:
  - name: component
    charts:
      - name: ../escaped
        namespace: default
        releaseName: valid
        local:
          path: chart
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, layout.ZarfYAML), []byte(packageYAML), 0o600))

	_, err := PackageDefinition(testutil.TestContext(t), dir, DefinitionOptions{})
	require.ErrorContains(t, err, "linting error found")
}

func TestPackageDefinitionRejectsUnsupportedRawFields(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		apiVersion string
	}{
		{
			name: "v1alpha1",
		},
		{
			name:       "v1beta1",
			apiVersion: "apiVersion: zarf.dev/v1beta1\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			packageYAML := tt.apiVersion + `kind: ZarfPackageConfig
metadata:
  name: raw-schema-error
components:
  - name: component
documenttaion:
  zarf.cli.openvex.json: .vex/zarf.cli.openvex.json
`
			require.NoError(t, os.WriteFile(filepath.Join(dir, layout.ZarfYAML), []byte(packageYAML), 0o600))

			_, err := PackageDefinition(testutil.TestContext(t), dir, DefinitionOptions{})

			var lintErr *lint.LintError
			require.ErrorAs(t, err, &lintErr)
			require.NotEmpty(t, lintErr.Findings)
		})
	}
}

func TestPackageDefinitionValidatesSelectedDocument(t *testing.T) {
	t.Parallel()
	alpha := "apiVersion: zarf.dev/v1alpha1\nkind: ZarfPackageConfig\nmetadata:\n  name: alpha\ncomponents:\n  - name: component\n"
	beta := `apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: beta
  description: |+
    selected beta

components:
  - name: component
`
	future := "apiVersion: zarf.dev/v99\n"
	unknownField := "unsupported: true\n"
	for _, tt := range []struct {
		name       string
		definition string
		wantName   string
		wantError  bool
	}{
		{name: "alpha then beta", definition: alpha + "---\n" + beta, wantName: "beta"},
		{name: "beta then alpha", definition: beta + "---\n" + alpha, wantName: "beta"},
		{name: "future then alpha", definition: future + "---\n" + alpha, wantName: "alpha"},
		{name: "alpha then future", definition: alpha + "---\n" + future, wantName: "alpha"},
		{name: "unknown beta field after alpha", definition: alpha + "---\n" + beta + unknownField, wantName: "beta", wantError: true},
		{name: "unknown beta field before alpha", definition: beta + unknownField + "---\n" + alpha, wantName: "beta", wantError: true},
		{name: "unknown alpha field after future", definition: future + "---\n" + alpha + unknownField, wantName: "alpha", wantError: true},
		{name: "unknown alpha field before future", definition: alpha + unknownField + "---\n" + future, wantName: "alpha", wantError: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, layout.ZarfYAML), []byte(tt.definition), 0o600))
			pkg, err := PackageDefinition(testutil.TestContext(t), dir, DefinitionOptions{})
			if tt.wantError {
				var lintErr *lint.LintError
				require.ErrorAs(t, err, &lintErr)
				require.Equal(t, tt.wantName, lintErr.PackageName)
				require.Len(t, lintErr.Findings, 1)
				require.Contains(t, lintErr.Findings[0].Description, "unsupported")
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantName, pkg.Metadata.Name)
			if tt.wantName == "beta" {
				require.Equal(t, "selected beta\n\n", pkg.Metadata.Description)
			}
		})
	}
}

func TestPackageDefinitionValidatesV1Beta1RepositoryGitReferences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		url    string
		commit string
		valid  bool
	}{
		{
			name:   "accepts a SHA-1 commit and HTTPS repository URL",
			url:    "https://example.com/repository.git",
			commit: "524980951ff16e19dc25232e9aea8fd693989ba6",
			valid:  true,
		},
		{
			name:   "rejects a short commit SHA",
			url:    "https://example.com/repository.git",
			commit: "5249809",
		},
		{
			name:   "rejects a non-hex commit SHA",
			url:    "https://example.com/repository.git",
			commit: "gggggggggggggggggggggggggggggggggggggggg",
		},
		{
			name:   "rejects an invalid repository URL",
			url:    "not a URI",
			commit: "524980951ff16e19dc25232e9aea8fd693989ba6",
		},
		{
			name:   "accepts an SSH repository URL",
			url:    "ssh://git@example.com/organization/repository.git",
			commit: "524980951ff16e19dc25232e9aea8fd693989ba6",
			valid:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			packageYAML := fmt.Sprintf(`apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: repository-schema-validation
components:
  - name: component
    repositories:
      - url: %q
        ref:
          commit: %q
`, tt.url, tt.commit)
			require.NoError(t, os.WriteFile(filepath.Join(dir, layout.ZarfYAML), []byte(packageYAML), 0o600))

			_, err := PackageDefinition(testutil.TestContext(t), dir, DefinitionOptions{})
			if tt.valid {
				require.NoError(t, err)
				return
			}
			var lintErr *lint.LintError
			require.ErrorAs(t, err, &lintErr)
			require.Len(t, lintErr.Findings, 1)
		})
	}
}

func TestPackageDefinitionRejectsV1Beta1GitURLsWithEmbeddedReferences(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	packageYAML := `apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: git-url-validation
components:
  - name: component
    repositories:
      - url: https://example.com/repository.git@legacy-tag
    charts:
      - name: chart
        namespace: default
        git:
          url: https://example.com/charts.git@legacy-tag
          ref:
            tag: v1.0.0
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, layout.ZarfYAML), []byte(packageYAML), 0o600))

	_, err := PackageDefinition(testutil.TestContext(t), dir, DefinitionOptions{})
	require.ErrorContains(t, err, "git URL \"https://example.com/repository.git@legacy-tag\" must not contain an embedded ref")
	require.ErrorContains(t, err, "git URL \"https://example.com/charts.git@legacy-tag\" must not contain an embedded ref")
}

func TestPackageUsesFlavor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		pkg      v1alpha1.ZarfPackage
		flavor   string
		expected bool
	}{
		{
			name: "when flavor is not set",
			pkg: v1alpha1.ZarfPackage{
				Components: []v1alpha1.ZarfComponent{
					{
						Name: "do-nothing",
					},
					{
						Name: "do-nothing-flavored",
						Only: v1alpha1.ZarfComponentOnlyTarget{
							Flavor: "cashew",
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "when flavor is not used",
			pkg: v1alpha1.ZarfPackage{
				Components: []v1alpha1.ZarfComponent{
					{
						Name: "do-nothing",
					},
				},
			},
			flavor:   "cashew",
			expected: false,
		},
		{
			name: "when flavor is used",
			pkg: v1alpha1.ZarfPackage{
				Components: []v1alpha1.ZarfComponent{
					{
						Name: "do-nothing",
						Only: v1alpha1.ZarfComponentOnlyTarget{
							Flavor: "cashew",
						},
					},
				},
			},
			flavor:   "cashew",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.expected, hasFlavoredComponent(tt.pkg, tt.flavor))
		})
	}
}

func TestPackageDefinitionWithValuesSchema(t *testing.T) {
	t.Parallel()

	_ = feature.Set([]feature.Feature{{Name: feature.Values, Enabled: true}}) //nolint:errcheck // Features may already be set by another test.

	tests := []struct {
		name        string
		packagePath string
		expectedErr string
	}{
		{
			name:        "valid values pass schema validation",
			packagePath: filepath.Join("testdata", "package-with-values-schema"),
		},
		{
			name:        "invalid values fail schema validation",
			packagePath: filepath.Join("testdata", "package-with-invalid-values"),
			expectedErr: "values validation failed",
		},
		{
			name:        "v1beta1 imported values fail imported schema validation",
			packagePath: filepath.Join("testdata", "v1beta1-imported-values-schema-invalid"),
			expectedErr: "values validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := testutil.TestContext(t)
			loaded, err := Package(ctx, tt.packagePath, PackageOptions{})
			if loaded != nil {
				t.Cleanup(func() { require.NoError(t, loaded.Close()) })
			}
			if tt.expectedErr != "" {
				require.ErrorContains(t, err, tt.expectedErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestV1Beta1PackageDefinition(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	t.Run("loads and validates, exposing both a v1alpha1 and a faithful v1beta1 view", func(t *testing.T) {
		t.Parallel()
		defined, err := PackageDefinition(ctx, filepath.Join("testdata", "v1beta1-package"), DefinitionOptions{})
		require.NoError(t, err)
		require.Equal(t, v1beta1.APIVersion, defined.APIVersion)

		pkg := convert.PackageToV1alpha1(defined)
		require.Equal(t, v1alpha1.APIVersion, pkg.APIVersion)
		require.Equal(t, "beta-package", pkg.Metadata.Name)
		require.NotEmpty(t, pkg.Metadata.Architecture)
		require.Len(t, pkg.Components, 1)
		require.Equal(t, "first", pkg.Components[0].Name)
		require.Equal(t, []string{"nginx:1.27.0"}, pkg.Components[0].Images)
		require.Equal(t, []string{"https://github.com/zarf-dev/zarf.git"}, pkg.Components[0].Repos)

		// The v1beta1 view preserves fields with no v1alpha1 representation — here an image's source.
		// Collapsing to v1alpha1 on load (the previous approach) dropped these.
		betaPkg := convert.PackageToV1beta1(defined)
		require.Equal(t, v1beta1.APIVersion, betaPkg.APIVersion)
		require.Len(t, betaPkg.Components, 1)
		require.Equal(t, "nginx:1.27.0", betaPkg.Components[0].Images[0].Name)
		require.Equal(t, v1beta1.ImageSourceDaemon, betaPkg.Components[0].Images[0].Source)
	})

	t.Run("resolves a local component config import", func(t *testing.T) {
		t.Parallel()
		defined, err := PackageDefinition(ctx, filepath.Join("testdata", "v1beta1-with-import"), DefinitionOptions{})
		require.NoError(t, err)

		pkg := convert.PackageToV1alpha1(defined)
		require.Equal(t, v1alpha1.APIVersion, pkg.APIVersion)
		require.Len(t, pkg.Components, 1)
		require.Equal(t, "imported", pkg.Components[0].Name)
		require.Equal(t, []string{"nginx:1.27.0"}, pkg.Components[0].Images)
	})
}

func TestV1Beta1PackageDefinitionValuesSchemaValidation(t *testing.T) {
	t.Parallel()
	_ = feature.Set([]feature.Feature{{Name: feature.Values, Enabled: true}}) //nolint:errcheck // Features may already be set by another test.
	ctx := testutil.TestContext(t)
	dir := filepath.Join("testdata", "v1beta1-invalid-values")

	_, err := PackageDefinition(ctx, dir, DefinitionOptions{})
	require.NoError(t, err)

	_, err = Package(ctx, dir, PackageOptions{})
	require.ErrorContains(t, err, "values validation failed")

	loaded, err := Package(ctx, dir, PackageOptions{SkipValuesSchemaValidation: true})
	require.NoError(t, err)
	require.NoError(t, loaded.Close())
}

func TestPackageDefinitionDoesNotAccessSourceResources(t *testing.T) {
	ctx := testutil.TestContext(t)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, layout.ZarfYAML), []byte(`apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: definition-only
values:
  files:
    - missing-values.yaml
components:
  - name: component
`), 0o600))

	definition, err := PackageDefinition(ctx, dir, DefinitionOptions{})
	require.NoError(t, err)
	require.Equal(t, "definition-only", convert.PackageToV1alpha1(definition).Metadata.Name)

	_, err = Package(ctx, dir, PackageOptions{})
	require.ErrorContains(t, err, "unable to access local resource \"missing-values.yaml\"")
}

func TestLoadedPackageCloseInvalidatesResources(t *testing.T) {
	ctx := testutil.TestContext(t)
	loaded, err := Package(ctx, filepath.Join("testdata", "v1beta1-package"), PackageOptions{})
	require.NoError(t, err)
	require.NoError(t, loaded.Close())
	require.NoError(t, loaded.Close())

	_, err = loaded.Resources.Root()
	require.ErrorContains(t, err, "package resources are closed")
}

func TestV1Beta1PackageDefinitionRejectsConflictingChartSources(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "child.yaml"), []byte(`apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: child
component:
  charts:
    - name: app
      namespace: app
      local:
        path: chart
`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, layout.ZarfYAML), []byte(`apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: source-replacement
components:
  - name: app
    import:
      local:
        - path: child.yaml
    charts:
      - name: app
        namespace: app
        oci:
          url: oci://example.com/chart
          ref:
            tag: 1.0.0
`), 0o600))

	_, err := PackageDefinition(ctx, dir, DefinitionOptions{})
	require.ErrorContains(t, err, `chart "app" must include exactly one source`)
}

func TestPackageDefinitionErrors(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	t.Run("returns error for non-existent package path", func(t *testing.T) {
		t.Parallel()
		_, err := PackageDefinition(ctx, filepath.Join(t.TempDir(), "does-not-exist"), DefinitionOptions{})
		require.Error(t, err)
	})

	t.Run("returns error when zarf.yaml contains invalid YAML", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "zarf.yaml"), []byte("this: is: not: valid: yaml: ["), 0o600))
		_, err := PackageDefinition(ctx, dir, DefinitionOptions{})
		require.Error(t, err)
	})

	t.Run("returns error when a component import path does not exist", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		zarfYAML := `kind: ZarfPackageConfig
metadata:
  name: test
components:
  - name: test
    import:
      path: ./does-not-exist
`
		require.NoError(t, os.WriteFile(filepath.Join(dir, "zarf.yaml"), []byte(zarfYAML), 0o600))
		_, err := PackageDefinition(ctx, dir, DefinitionOptions{})
		require.ErrorContains(t, err, "does-not-exist")
	})

	t.Run("returns error when a required package template variable is not set", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		zarfYAML := `kind: ZarfPackageConfig
metadata:
  name: "###ZARF_PKG_TMPL_MYVAR###"
components:
  - name: "###ZARF_PKG_TMPL_MYVAR###"
    required: true
    actions:
      onCreate:
        before:
          - cmd: "###ZARF_PKG_TMPL_MYVAR###"
`
		require.NoError(t, os.WriteFile(filepath.Join(dir, "zarf.yaml"), []byte(zarfYAML), 0o600))
		_, err := PackageDefinition(ctx, dir, DefinitionOptions{
			SetVariables: map[string]string{}, // non-nil triggers fillActiveTemplate; MYVAR is absent
		})
		require.ErrorContains(t, err, `template "MYVAR" must be '--set' when using the '--confirm' flag`)
	})

	t.Run("resolves package and component names from package templates", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		zarfYAML := `kind: ZarfPackageConfig
metadata:
  name: "###ZARF_PKG_TMPL_MYVAR###"
components:
  - name: "###ZARF_PKG_TMPL_MYVAR###"
    required: true
`
		require.NoError(t, os.WriteFile(filepath.Join(dir, "zarf.yaml"), []byte(zarfYAML), 0o600))

		pkg, err := PackageDefinition(ctx, dir, DefinitionOptions{
			SetVariables: map[string]string{"MYVAR": "test-package"},
		})
		require.NoError(t, err)
		require.Equal(t, "test-package", pkg.Metadata.Name)
		require.Equal(t, "test-package", pkg.Components[0].Name)
	})
}
