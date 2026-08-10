// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	gotemplate "text/template"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/parser"
	"github.com/spf13/cobra"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/value"
)

const packageTemplateFilename = "zarf.tpl.yaml"

type devTemplateOptions struct {
	set     map[string]string
	setFile string
}

// templateConfig is the small portion of a v1beta1 definition needed to find local component imports.
// It intentionally does not validate the full definition: validation belongs to the commands that consume
// the generated definition.
type templateConfig struct {
	APIVersion string              `yaml:"apiVersion"`
	Kind       v1beta1.PackageKind `yaml:"kind"`
	Components []templateComponent `yaml:"components"`
	Component  templateComponent   `yaml:"component"`
}

type templateComponent struct {
	Import templateImport `yaml:"import"`
}

type templateImport struct {
	Local []templateImportLocal `yaml:"local"`
}

type templateImportLocal struct {
	Path string `yaml:"path"`
}

type templateLocalImport struct {
	path     string
	yamlPath string
}

type packageTemplateRenderer struct {
	values   value.Values
	outputs  map[string][]byte
	done     map[string]bool
	visiting map[string]bool
}

func newDevTemplateCommand() *cobra.Command {
	o := &devTemplateOptions{}
	cmd := &cobra.Command{
		Use:     "template [ TEMPLATE_FILE | DIRECTORY ]",
		Short:   "Renders a package template into a generated package definition",
		Example: "zarf dev template --set ENVIRONMENT=personal --set MY_IMAGE=ghcr.io/my-org/my-image:0.0.1",
		Hidden:  true,
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return o.run(cmd.Context(), args)
		},
	}
	cmd.Flags().StringToStringVar(&o.set, "set", nil, "Set a package template value (key=value)")
	cmd.Flags().StringVar(&o.setFile, "set-file", "", "YAML file containing package template values")
	return cmd
}

func (o *devTemplateOptions) run(ctx context.Context, args []string) error {
	source, err := templateSourcePath(args)
	if err != nil {
		return err
	}

	valuesFiles := []string(nil)
	if o.setFile != "" {
		if _, err := os.Stat(o.setFile); err != nil {
			return fmt.Errorf("unable to access template values file %q: %w", o.setFile, err)
		}
		valuesFiles = []string{o.setFile}
	}
	values, err := parseValues(ctx, valuesFiles, o.set)
	if err != nil {
		return fmt.Errorf("unable to parse package template values: %w", err)
	}
	// cli is reserved so templates can reliably use [[ .cli.version ]] to create versioned init packages.
	values["cli"] = map[string]any{"version": config.CLIVersion}

	renderer := packageTemplateRenderer{
		values:   values,
		outputs:  make(map[string][]byte),
		done:     make(map[string]bool),
		visiting: make(map[string]bool),
	}
	if err := renderer.render(source); err != nil {
		return err
	}
	for output, rendered := range renderer.outputs {
		if err := os.WriteFile(output, rendered, 0o644); err != nil {
			return fmt.Errorf("writing generated template %q: %w", output, err)
		}
	}
	return nil
}

func templateSourcePath(args []string) (string, error) {
	path := packageTemplateFilename
	if len(args) == 1 {
		path = args[0]
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("unable to access package template %q: %w", path, err)
	}
	if info.IsDir() {
		path = filepath.Join(path, packageTemplateFilename)
	}
	if !strings.HasSuffix(path, ".tpl.yaml") {
		return "", fmt.Errorf("package template %q must end in .tpl.yaml", path)
	}
	return filepath.Abs(path)
}

func (r *packageTemplateRenderer) render(source string) error {
	if r.done[source] {
		return nil
	}
	if r.visiting[source] {
		return fmt.Errorf("package template %q imports itself in a cycle", source)
	}
	r.visiting[source] = true
	defer delete(r.visiting, source)

	contents, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("reading package template %q: %w", source, err)
	}
	rendered, err := executePackageTemplate(source, contents, r.values)
	if err != nil {
		return err
	}

	imports, err := localTemplateImports(source, rendered)
	if err != nil {
		return err
	}
	replacements := make(map[string]string)
	for _, imported := range imports {
		if !strings.HasSuffix(imported.path, ".tpl.yaml") {
			continue
		}
		importSource := imported.path
		if !filepath.IsAbs(importSource) {
			importSource = filepath.Join(filepath.Dir(source), importSource)
		}
		importSource = filepath.Clean(importSource)
		if err := r.render(importSource); err != nil {
			return err
		}
		replacements[imported.yamlPath] = strings.TrimSuffix(imported.path, ".tpl.yaml") + ".gen.yaml"
	}
	if len(replacements) > 0 {
		rendered, err = rewriteTemplateImports(rendered, replacements)
		if err != nil {
			return fmt.Errorf("rewriting local imports in package template %q: %w", source, err)
		}
	}

	r.outputs[generatedTemplatePath(source)] = rendered
	r.done[source] = true
	return nil
}

func generatedTemplatePath(source string) string {
	return strings.TrimSuffix(source, ".tpl.yaml") + ".gen.yaml"
}

func executePackageTemplate(path string, contents []byte, values value.Values) ([]byte, error) {
	tmpl, err := gotemplate.New(filepath.Base(path)).Delims("[[", "]]").Option("missingkey=error").Parse(string(contents))
	if err != nil {
		return nil, fmt.Errorf("parsing package template %q: %w", path, err)
	}
	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, values); err != nil {
		return nil, fmt.Errorf("rendering package template %q: %w", path, err)
	}
	return rendered.Bytes(), nil
}

func localTemplateImports(source string, rendered []byte) ([]templateLocalImport, error) {
	var config templateConfig
	if err := yaml.Unmarshal(rendered, &config); err != nil {
		return nil, fmt.Errorf("parsing rendered package template %q: %w", source, err)
	}
	if config.APIVersion != v1beta1.APIVersion {
		return nil, fmt.Errorf("package template %q must use apiVersion %q", source, v1beta1.APIVersion)
	}

	var imports []templateLocalImport
	switch config.Kind {
	case "", v1beta1.ZarfPackageConfig:
		for componentIdx, component := range config.Components {
			for importIdx, entry := range component.Import.Local {
				imports = append(imports, templateLocalImport{
					path:     entry.Path,
					yamlPath: fmt.Sprintf("$.components[%d].import.local[%d].path", componentIdx, importIdx),
				})
			}
		}
	case v1beta1.ZarfComponentConfig:
		for importIdx, entry := range config.Component.Import.Local {
			imports = append(imports, templateLocalImport{
				path:     entry.Path,
				yamlPath: fmt.Sprintf("$.component.import.local[%d].path", importIdx),
			})
		}
	default:
		return nil, fmt.Errorf("package template %q has unsupported kind %q", source, config.Kind)
	}
	return imports, nil
}

func rewriteTemplateImports(rendered []byte, replacements map[string]string) ([]byte, error) {
	file, err := parser.ParseBytes(rendered, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	for yamlPath, replacement := range replacements {
		path, err := yaml.PathString(yamlPath)
		if err != nil {
			return nil, err
		}
		node, err := yaml.ValueToNode(replacement)
		if err != nil {
			return nil, err
		}
		if err := path.ReplaceWithNode(file, node); err != nil {
			return nil, err
		}
	}
	return []byte(file.String()), nil
}
