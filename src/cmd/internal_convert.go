// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	goyaml "github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
	"github.com/zarf-dev/zarf/src/api/convert"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
)

type internalConvertOptions struct{}

// FIXME: temporary internal convert command
func newInternalConvertCommand() *cobra.Command {
	o := &internalConvertOptions{}

	cmd := &cobra.Command{
		Use:   "convert [directory]",
		Short: "Convert a v1alpha1 zarf.yaml to v1beta1",
		Args:  cobra.ExactArgs(1),
		RunE:  o.run,
	}

	return cmd
}

func (o *internalConvertOptions) run(_ *cobra.Command, args []string) error {
	dir := args[0]

	inputPath := filepath.Join(dir, "zarf.yaml")
	b, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", inputPath, err)
	}

	var pkg v1alpha1.ZarfPackage
	if err := goyaml.Unmarshal(b, &pkg); err != nil {
		return fmt.Errorf("parsing %s: %w", inputPath, err)
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

	fmt.Printf("Converted %s -> %s\n", inputPath, outputPath)
	return nil
}
