// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	gotemplate "text/template"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/pkg/value"
)

const packageTemplateFilename = "zarf.tpl.yaml"

type devTemplateOptions struct {
	set            map[string]string
	setFile        string
	skipValidation bool
}

func newDevTemplateCommand(v *viper.Viper) *cobra.Command {
	o := &devTemplateOptions{}
	cmd := &cobra.Command{
		Use:   "template [ TEMPLATE_FILE | DIRECTORY ]",
		Short: "Renders a template into a generated definition",
		// Once v1beta1 is released this command should be unhidden
		Hidden: true,
		Args:   cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return o.run(cmd.Context(), args)
		},
	}
	cmd.Flags().StringToStringVar(&o.set, "set", v.GetStringMapString(VDevTemplateSet), "Set a package template value (key=value)")
	cmd.Flags().StringVar(&o.setFile, "set-file", v.GetString(VDevTemplateSetFile), "YAML file containing package template values")
	cmd.Flags().BoolVar(&o.skipValidation, "skip-validation", v.GetBool(VDevTemplateSkipValidation), "Skip schema validation of the rendered definition")
	return cmd
}

func (o *devTemplateOptions) run(ctx context.Context, args []string) error {
	basePath, err := setBaseDirectory(args)
	if err != nil {
		return err
	}

	source, err := templateSourcePath(basePath)
	if err != nil {
		return err
	}

	valuesFiles := []string{}
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
	values["cli"] = new(value.Values{"version": config.CLIVersion})

	contents, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("reading package template %q: %w", source, err)
	}
	rendered, err := executePackageTemplate(source, contents, values)
	if err != nil {
		return err
	}
	kind, err := validateTemplateAPIVersion(source, rendered)
	if err != nil {
		return err
	}
	if !o.skipValidation {
		if err := validateTemplateSchema(source, rendered, kind); err != nil {
			if lintErr, ok := errors.AsType[*lint.LintError](err); ok {
				PrintFindings(ctx, lintErr)
			}
			return err
		}
	}
	output := generatedTemplatePath(source)
	if err := os.WriteFile(output, rendered, 0o644); err != nil {
		return fmt.Errorf("writing generated template %q: %w", output, err)
	}
	return nil
}

func templateSourcePath(path string) (string, error) {
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
	return path, nil
}

func generatedTemplatePath(source string) string {
	return strings.TrimSuffix(source, ".tpl.yaml") + ".gen.yaml"
}

func validateTemplateAPIVersion(path string, rendered []byte) (v1beta1.PackageKind, error) {
	var definition struct {
		APIVersion string              `yaml:"apiVersion"`
		Kind       v1beta1.PackageKind `yaml:"kind"`
	}
	if err := yaml.Unmarshal(rendered, &definition); err != nil {
		return "", fmt.Errorf("parsing rendered package template %q: %w", path, err)
	}
	if definition.APIVersion != v1beta1.APIVersion {
		return "", fmt.Errorf("package template %q must use apiVersion %q", path, v1beta1.APIVersion)
	}
	return definition.Kind, nil
}

func validateTemplateSchema(path string, rendered []byte, kind v1beta1.PackageKind) error {
	var (
		findings []lint.PackageFinding
		err      error
	)
	switch kind {
	case "", v1beta1.ZarfPackageConfig:
		findings, err = lint.ValidatePackageSchemaBytesV1Beta1(rendered)
	case v1beta1.ZarfComponentConfig:
		findings, err = lint.ValidateComponentConfigSchemaBytesV1Beta1(rendered)
	default:
		return fmt.Errorf("package template %q has unsupported kind %q", path, kind)
	}
	if err != nil {
		return fmt.Errorf("validating rendered package template %q: %w", path, err)
	}
	if len(findings) > 0 {
		return &lint.LintError{PackageName: path, Findings: findings}
	}
	return nil
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
