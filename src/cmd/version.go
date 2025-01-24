// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"runtime/debug"

	"github.com/Masterminds/semver/v3"
	goyaml "github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
)

type versionOptions struct {
	outputFormat string
}

func newVersionCommand() *cobra.Command {
	o := versionOptions{}

	cmd := &cobra.Command{
		Use:     "version",
		Aliases: []string{"v"},
		Short:   lang.CmdVersionShort,
		Long:    lang.CmdVersionLong,
		RunE:    o.run,
	}

	cmd.Flags().StringVarP(&o.outputFormat, "output", "o", "", "Output format (yaml|json)")

	return cmd
}

func (o *versionOptions) run(_ *cobra.Command, _ []string) error {
	if o.outputFormat == "" {
		fmt.Println(config.CLIVersion)
		return nil
	}

	output := make(map[string]interface{})
	output["version"] = config.CLIVersion

	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return errors.New("failed to get build info")
	}
	depMap := map[string]string{}
	for _, dep := range buildInfo.Deps {
		if dep.Replace != nil {
			depMap[dep.Path] = fmt.Sprintf("%s -> %s %s", dep.Version, dep.Replace.Path, dep.Replace.Version)
		} else {
			depMap[dep.Path] = dep.Version
		}
	}
	output["dependencies"] = depMap

	buildMap := make(map[string]interface{})
	buildMap["platform"] = runtime.GOOS + "/" + runtime.GOARCH
	buildMap["goVersion"] = runtime.Version()
	ver, err := semver.NewVersion(config.CLIVersion)
	if err != nil && !errors.Is(err, semver.ErrInvalidSemVer) {
		return fmt.Errorf("Could not parse CLI version %s: %w", config.CLIVersion, err)
	}
	if ver != nil {
		buildMap["major"] = ver.Major()
		buildMap["minor"] = ver.Minor()
		buildMap["patch"] = ver.Patch()
		buildMap["prerelease"] = ver.Prerelease()
	}
	output["build"] = buildMap

	switch o.outputFormat {
	case "yaml":
		b, err := goyaml.Marshal(output)
		if err != nil {
			return fmt.Errorf("could not marshal yaml output: %w", err)
		}
		fmt.Println(string(b))
	case "json":
		b, err := json.Marshal(output)
		if err != nil {
			return fmt.Errorf("could not marshal json output: %w", err)
		}
		fmt.Println(string(b))
	}

	return nil
}
