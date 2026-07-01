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
	"github.com/zarf-dev/zarf/src/api/v1beta1"
)

var supportedAPIVersions = []string{v1alpha1.APIVersion, v1beta1.APIVersion}

type devUpgradeSchemaOptions struct {
	to string
}

// This command will be unhidden once v1beta1 is ready for use
func newDevUpgradeSchemaCommand() *cobra.Command {
	o := &devUpgradeSchemaOptions{}

	cmd := &cobra.Command{
		Use:     "upgrade-schema [ DIRECTORY ]",
		Short:   "Converts and outputs the existing zarf package config to the given API version. Defaults to latest API version.",
		Example: "zarf dev upgrade-schema . > zarf.yaml",
		Hidden:  true,
		Args:    cobra.MaximumNArgs(1),
		RunE:    o.run,
	}

	cmd.Flags().StringVar(&o.to, "to", v1beta1.APIVersion, "Specify the API version to upgrade the package definition to.")

	return cmd
}

func (o *devUpgradeSchemaOptions) run(cmd *cobra.Command, args []string) error {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}

	inputPath := filepath.Join(dir, "zarf.yaml")
	b, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", inputPath, err)
	}

	sourceVersion, err := detectAPIVersion(b)
	if err != nil {
		return err
	}

	if err := validateVersionUpgrade(sourceVersion, o.to); err != nil {
		return err
	}

	if sourceVersion == o.to {
		if _, err := fmt.Fprint(cmd.OutOrStdout(), string(b)); err != nil {
			return fmt.Errorf("writing package: %w", err)
		}
		return nil
	}

	switch {
	case sourceVersion == v1alpha1.APIVersion && o.to == v1beta1.APIVersion:
		var pkg v1alpha1.ZarfPackage
		if err := goyaml.Unmarshal(b, &pkg); err != nil {
			return fmt.Errorf("parsing %s: %w", inputPath, err)
		}
		if err := checkRemovedFields(pkg); err != nil {
			return err
		}
		result := convert.PackageV1alpha1ToV1beta1(pkg)
		out, err := goyaml.Marshal(result)
		if err != nil {
			return fmt.Errorf("marshaling %s package: %w", o.to, err)
		}
		if _, err := fmt.Fprint(cmd.OutOrStdout(), string(out)); err != nil {
			return fmt.Errorf("writing converted package: %w", err)
		}
	default:
		return fmt.Errorf("unsupported conversion from %s to %s", sourceVersion, o.to)
	}

	return nil
}

func detectAPIVersion(b []byte) (string, error) {
	var header struct {
		APIVersion string `yaml:"apiVersion"`
	}
	if err := goyaml.Unmarshal(b, &header); err != nil {
		return "", fmt.Errorf("reading apiVersion: %w", err)
	}
	if header.APIVersion == "" {
		return v1alpha1.APIVersion, nil
	}
	return header.APIVersion, nil
}

func validateVersionUpgrade(from, to string) error {
	fromIdx := -1
	toIdx := -1
	for i, v := range supportedAPIVersions {
		if v == from {
			fromIdx = i
		}
		if v == to {
			toIdx = i
		}
	}
	if fromIdx == -1 {
		return fmt.Errorf("unsupported source API version %q", from)
	}
	if toIdx == -1 {
		return fmt.Errorf("unsupported target API version %q", to)
	}
	if toIdx < fromIdx {
		return fmt.Errorf("cannot downgrade from %s to %s", from, to)
	}
	return nil
}

func checkRemovedFields(pkg v1alpha1.ZarfPackage) error {
	var errs []error
	if pkg.Metadata.YOLO {
		// TODO, add link to connected docs when available
		errs = append(errs, fmt.Errorf(".metadata.yolo is removed without replacement in v1beta1 — replace it with connected deployments"))
	}
	// TODO link to values docs
	if len(pkg.Variables) > 0 {
		errs = append(errs, fmt.Errorf(".variables is removed in v1beta1 — consider using Zarf values instead"))
	}
	if len(pkg.Constants) > 0 {
		errs = append(errs, fmt.Errorf(".constants is removed in v1beta1 — consider using Zarf values instead"))
	}
	for _, c := range pkg.Components {
		if c.DeprecatedGroup != "" {
			errs = append(errs, fmt.Errorf("can't convert component %s, .components.group is removed without replacement in v1beta1 — consider using .components[x].only.flavor instead", c.Name))
		}
		if c.Default {
			errs = append(errs, fmt.Errorf("can't convert component %s, .components.default is removed without replacement in v1beta1", c.Name))
		}
		if len(c.DataInjections) > 0 {
			errs = append(errs, fmt.Errorf("can't convert component %s, .components.dataInjections is removed without replacement in v1beta1 — see https://docs.zarf.dev/best-practices/data-injections-migration/ for alternatives", c.Name))
		}
		if len(c.Only.Cluster.Distros) > 0 {
			errs = append(errs, fmt.Errorf("can't convert component %s, .components.only.cluster.distro is removed without replacement in v1beta1", c.Name))
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
		errs = append(errs, checkRemovedActionFields(c)...)
	}
	return errors.Join(errs...)
}

// checkRemovedActionFields reports actions using setVariable/setVariables, which are removed in v1beta1 in favor of setValues.
func checkRemovedActionFields(c v1alpha1.ZarfComponent) []error {
	var errs []error
	actionSets := []struct {
		onAny string
		set   v1alpha1.ZarfComponentActionSet
	}{
		{"onCreate", c.Actions.OnCreate},
		{"onDeploy", c.Actions.OnDeploy},
		{"onRemove", c.Actions.OnRemove},
	}
	for _, as := range actionSets {
		set := as.set
		for _, actions := range [][]v1alpha1.ZarfComponentAction{set.Before, set.After, set.OnSuccess, set.OnFailure} {
			for _, a := range actions {
				if a.DeprecatedSetVariable != "" || len(a.SetVariables) > 0 {
					errs = append(errs, fmt.Errorf("can't convert component %s, .components.actions.%s setVariable/setVariables is removed in v1beta1 — use setValues instead", c.Name, as.onAny))
					break
				}
			}
		}
	}
	return errs
}
