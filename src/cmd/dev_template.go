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

	"github.com/spf13/cobra"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/value"
)

const packageTemplateFilename = "zarf.tpl.yaml"

type devTemplateOptions struct {
	set     map[string]string
	setFile string
}

func newDevTemplateCommand() *cobra.Command {
	o := &devTemplateOptions{}
	cmd := &cobra.Command{
		Use:    "template [ TEMPLATE_FILE | DIRECTORY ]",
		Short:  "Renders a template into a generated definition",
		Hidden: true,
		Args:   cobra.MaximumNArgs(1),
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

	contents, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("reading package template %q: %w", source, err)
	}
	rendered, err := executePackageTemplate(source, contents, values)
	if err != nil {
		return err
	}
	output := generatedTemplatePath(source)
	if err := os.WriteFile(output, rendered, 0o644); err != nil {
		return fmt.Errorf("writing generated template %q: %w", output, err)
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
