// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
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
func (p *Packager) Generate(name, url, version, gitPath, outputDir, kubeVersionOverride string) (err error) {
	generatedZarfYAMLPath := filepath.Join(outputDir, layout.ZarfYAML)
	spinner := message.NewProgressSpinner("Generating package for %q at %s", name, generatedZarfYAMLPath)

	if !helpers.InvalidPath(generatedZarfYAMLPath) {
		prefixed := filepath.Join(outputDir, fmt.Sprintf("%s-%s", name, layout.ZarfYAML))

		message.Warnf("%s already exists, writing to %s", generatedZarfYAMLPath, prefixed)

		generatedZarfYAMLPath = prefixed

		if !helpers.InvalidPath(generatedZarfYAMLPath) {
			return fmt.Errorf("unable to generate package, %s already exists", generatedZarfYAMLPath)
		}
	}

	generatedComponent := types.ZarfComponent{
		Name:     name,
		Required: helpers.BoolPtr(true),
		Charts: []types.ZarfChart{
			{
				Name:      name,
				Version:   version,
				Namespace: name,
				URL:       url,
				GitPath:   gitPath,
			},
		},
	}

	p.cfg.Pkg = types.ZarfPackage{
		Kind: types.ZarfPackageConfig,
		Metadata: types.ZarfMetadata{
			Name:        name,
			Version:     version,
			Description: "auto-generated using `zarf dev generate`",
		},
		Components: []types.ZarfComponent{
			generatedComponent,
		},
	}

	images, err := p.findImages(gitPath, false, kubeVersionOverride, "", "")
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

	if err := helpers.CreateDirectory(outputDir, helpers.ReadExecuteAllWriteUser); err != nil {
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

	spinner.Successf("Generated package for %q at %s", name, generatedZarfYAMLPath)

	return os.WriteFile(generatedZarfYAMLPath, []byte(content), helpers.ReadAllWriteUser)
}
