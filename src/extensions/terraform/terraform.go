// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package terraform contains the logic for downloading and setting up Terraform
package terraform

import (
	"fmt"
	"runtime"

	"github.com/defenseunicorns/zarf/src/types"
)

// Run mutates a component that should deploy Terraform
func Run(_ bool, arch string, _ types.ComponentPaths, c types.ZarfComponent) (types.ZarfComponent, error) {
	if c.Only.LocalOS == "" {
		return c, fmt.Errorf("the terraform extension requires that you set only.localOS to pull the correct terraform binary for this system")
	}

	terraformVersion := c.Extensions.Terraform.Version
	terraformFilePath := fmt.Sprintf("terraform_%s_%s_%s", terraformVersion, c.Only.LocalOS, arch)
	terraformUrl := fmt.Sprintf("https://releases.hashicorp.com/terraform/%s/%s.zip", terraformVersion, terraformFilePath)
	terraformDst := fmt.Sprintf("~/.zarf-cache/bin/%s", terraformFilePath)

	prepareCmd := fmt.Sprintf("rm -r -f %s %s.zip && mkdir -p ~/.zarf-cache/bin", terraformDst, terraformDst)
	prepareAction := types.ZarfComponentAction{
		Cmd: prepareCmd,
	}

	downloadCmd := fmt.Sprintf("curl %s -o %s.zip", terraformUrl, terraformDst)
	downloadAction := types.ZarfComponentAction{
		Cmd: downloadCmd,
	}

	decompressCmd := fmt.Sprintf("./zarf tools archiver decompress %s.zip %s", terraformDst, terraformDst)
	decompressAction := types.ZarfComponentAction{
		Cmd: decompressCmd,
	}

	c.Actions.OnCreate.Before = append(c.Actions.OnCreate.Before, prepareAction, downloadAction, decompressAction)

	if c.Only.LocalOS == runtime.GOOS {
		terraformGetCmd := fmt.Sprintf("%s/terraform get", terraformDst)
		terraformGetAction := types.ZarfComponentAction{
			Cmd: terraformGetCmd,
			Dir: &c.Extensions.Terraform.Source,
		}

		c.Actions.OnCreate.Before = append(c.Actions.OnCreate.Before, terraformGetAction)
	}

	fileExtension := ""
	if c.Only.LocalOS == "windows" {
		fileExtension = ".exe"
	}

	terraformBinary := types.ZarfFile{
		Source: fmt.Sprintf("%s/terraform%s", terraformDst, fileExtension),
		Target: "~/.zarf/bin/terraform",
	}

	terraformFiles := types.ZarfFile{
		Source: c.Extensions.Terraform.Source,
		Target: fmt.Sprintf(".terraform/%s", c.Name),
	}

	c.Files = append(c.Files, terraformBinary, terraformFiles)

	return c, nil
}

// Skeletonize mutates a component so that the terraform files can be contained inside a skeleton package
// func Skeletonize(tmpPaths types.ComponentPaths, c types.ZarfComponent) (types.ZarfComponent, error) {

// }

// Compose mutates a component so that the valuesFiles are relative to the parent importing component
// func Compose(pathAncestry string, c types.ZarfComponent) types.ZarfComponent {

// }
