// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"runtime"
	"runtime/debug"

	"github.com/Masterminds/semver/v3"
	goyaml "github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/message"
)

type versionOptions struct {
	outputFormat outputFormat
	outputWriter io.Writer
}

func newVersionOptions() *versionOptions {
	return &versionOptions{
		outputFormat: "",
		// TODO accept output writer as a parameter to the root Zarf command and pass it through here
		outputWriter: message.OutputWriter,
	}
}

func newVersionCommand() *cobra.Command {
	o := newVersionOptions()

	cmd := &cobra.Command{
		Use:     "version",
		Aliases: []string{"v"},
		Short:   lang.CmdVersionShort,
		Long:    lang.CmdVersionLong,
		RunE:    o.run,
	}

	cmd.Flags().VarP(&o.outputFormat, "output-format", "o", "Output format (yaml|json)")
	cmd.Flags().VarP(&o.outputFormat, "output", "", "Output format (yaml|json)")
	cmd.Flags().MarkDeprecated("output", "output is deprecated. Please use --output-format instead")

	return cmd
}

func (o *versionOptions) run(_ *cobra.Command, _ []string) error {
	if o.outputFormat == "" {
		fmt.Fprintln(o.outputWriter, config.CLIVersion)
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
		return fmt.Errorf("could not parse CLI version %s: %w", config.CLIVersion, err)
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
		fmt.Fprintln(o.outputWriter, string(b))
	case "json":
		b, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return fmt.Errorf("could not marshal json output: %w", err)
		}
		fmt.Fprintln(o.outputWriter, string(b))
	default:
		return fmt.Errorf("unsupported output format: %s", o.outputFormat)
	}
	return nil
}
