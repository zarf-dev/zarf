// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	goyaml "github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
	"github.com/zarf-dev/zarf/src/api/convert"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/logger"
)

type internalConvertOptions struct{}

// This command will be unhidden and moved to dev once v1beta1 is ready for use
func newInternalConvertCommand() *cobra.Command {
	o := &internalConvertOptions{}

	cmd := &cobra.Command{
		Use:    "convert [directory]",
		Short:  "Convert zarf.yaml to the latest API version (V1beta1)",
		Hidden: true,
		Args:   cobra.MaximumNArgs(1),
		RunE:   o.run,
	}

	return cmd
}

func (o *internalConvertOptions) run(cmd *cobra.Command, args []string) error {
	l := logger.From(cmd.Context())
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}

	inputPath := filepath.Join(dir, "zarf.yaml")
	b, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", inputPath, err)
	}

	var pkg v1alpha1.ZarfPackage
	if err := goyaml.Unmarshal(b, &pkg); err != nil {
		return fmt.Errorf("parsing %s: %w", inputPath, err)
	}

	if err := checkRemovedFields(pkg); err != nil {
		return err
	}

	result := convert.V1Alpha1PkgToV1Beta1(pkg)

	out, err := goyaml.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshaling v1beta1 package: %w", err)
	}

	outputPath := filepath.Join(dir, "zarf-v1beta1.yaml")
	if err := os.WriteFile(outputPath, out, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", outputPath, err)
	}

	l.Info("converted", "input", inputPath, "output", outputPath)
	return nil
}

func checkRemovedFields(pkg v1alpha1.ZarfPackage) error {
	var errs []error
	if pkg.Metadata.YOLO {
		// TODO, add link to connected docs when available
		errs = append(errs, fmt.Errorf(".metadata.yolo is removed without replacement in v1beta1 — replace it with connected deployments"))
	}
	for _, c := range pkg.Components {
		if c.DeprecatedGroup != "" {
			errs = append(errs, fmt.Errorf("can't convert component %s, .components.group is removed without replacement in v1beta1 — consider using .components[x].only.flavor instead", c.Name))
		}
		if len(c.DataInjections) > 0 {
			errs = append(errs, fmt.Errorf("can't convert component %s, .components.dataInjections is removed without replacement in v1beta1 — see https://docs.zarf.dev/best-practices/data-injections-migration/ for alternatives", c.Name))
		}
		// TODO add link to example of newer import system
		if c.Import.Name != "" {
			errs = append(errs, fmt.Errorf("can't convert component %s, .components.import.name is removed without replacement in v1beta1", c.Name))
		}
		for _, ch := range c.Charts {
			// TODO link to values docs
			if len(ch.Variables) > 0 {
				errs = append(errs, fmt.Errorf("can't convert chart %s in component %s, .components.charts.variables is removed without replacement in v1beta1 — consider using Zarf values instead", ch.Name, c.Name))
			}
		}
	}
	return errors.Join(errs...)
}
