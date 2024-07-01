// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/types"
	goyaml "github.com/goccy/go-yaml"
)

// Generate generates a Zarf package definition.
func (p *Packager) Generate(ctx context.Context) (err error) {
	generatedZarfYAMLPath := filepath.Join(p.cfg.GenerateOpts.Output, layout.ZarfYAML)
	spinner := message.NewProgressSpinner("Generating package for %q at %s", p.cfg.GenerateOpts.Name, generatedZarfYAMLPath)

	if !helpers.InvalidPath(generatedZarfYAMLPath) {
		prefixed := filepath.Join(p.cfg.GenerateOpts.Output, fmt.Sprintf("%s-%s", p.cfg.GenerateOpts.Name, layout.ZarfYAML))

		message.Warnf("%s already exists, writing to %s", generatedZarfYAMLPath, prefixed)

		generatedZarfYAMLPath = prefixed

		if !helpers.InvalidPath(generatedZarfYAMLPath) {
			return fmt.Errorf("unable to generate package, %s already exists", generatedZarfYAMLPath)
		}
	}

	generatedComponent := types.ZarfComponent{
		Name:     p.cfg.GenerateOpts.Name,
		Required: helpers.BoolPtr(true),
		Charts: []types.ZarfChart{
			{
				Name:      p.cfg.GenerateOpts.Name,
				Version:   p.cfg.GenerateOpts.Version,
				Namespace: p.cfg.GenerateOpts.Name,
				URL:       p.cfg.GenerateOpts.URL,
				GitPath:   p.cfg.GenerateOpts.GitPath,
			},
		},
	}

	p.cfg.Pkg = types.ZarfPackage{
		Kind: types.ZarfPackageConfig,
		Metadata: types.ZarfMetadata{
			Name:        p.cfg.GenerateOpts.Name,
			Version:     p.cfg.GenerateOpts.Version,
			Description: "auto-generated using `zarf dev generate`",
		},
		Components: []types.ZarfComponent{
			generatedComponent,
		},
	}

	images, err := p.findImages(ctx)
	if err != nil {
		// purposefully not returning error here, as we can still generate the package without images
		message.Warnf("Unable to find images: %s", err.Error())
	}

	for i := range p.cfg.Pkg.Components {
		name := p.cfg.Pkg.Components[i].Name
		p.cfg.Pkg.Components[i].Images = images[name]
	}

	if err := p.cfg.Pkg.Validate(); err != nil {
		return err
	}

	if err := helpers.CreateDirectory(p.cfg.GenerateOpts.Output, helpers.ReadExecuteAllWriteUser); err != nil {
		return err
	}

	b, err := goyaml.MarshalWithOptions(p.cfg.Pkg, goyaml.IndentSequence(true), goyaml.UseSingleQuote(false))
	if err != nil {
		return err
	}

	schemaComment := fmt.Sprintf("# yaml-language-server: $schema=https://raw.githubusercontent.com/%s/%s/zarf.schema.json", config.GithubProject, config.CLIVersion)
	content := schemaComment + "\n" + string(b)

	// lets space things out a bit
	content = strings.Replace(content, "kind:\n", "\nkind:\n", 1)
	content = strings.Replace(content, "metadata:\n", "\nmetadata:\n", 1)
	content = strings.Replace(content, "components:\n", "\ncomponents:\n", 1)

	spinner.Successf("Generated package for %q at %s", p.cfg.GenerateOpts.Name, generatedZarfYAMLPath)

	return os.WriteFile(generatedZarfYAMLPath, []byte(content), helpers.ReadAllWriteUser)
}
