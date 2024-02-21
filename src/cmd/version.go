// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/Masterminds/semver/v3"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/spf13/cobra"

	"runtime/debug"

	goyaml "github.com/goccy/go-yaml"
)

var outputFormat string

var versionCmd = &cobra.Command{
	Use:     "version",
	Aliases: []string{"v"},
	PersistentPreRun: func(_ *cobra.Command, _ []string) {
		config.SkipLogFile = true
	},
	Short: lang.CmdVersionShort,
	Long:  lang.CmdVersionLong,
	Run: func(_ *cobra.Command, _ []string) {
		output := make(map[string]interface{})

		buildInfo, ok := debug.ReadBuildInfo()
		if !ok && outputFormat != "" {
			fmt.Println("Failed to get build info")
			return
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
		ver, _ := semver.NewVersion(config.CLIVersion)
		if ver != nil {
			buildMap["major"] = ver.Major()
			buildMap["minor"] = ver.Minor()
			buildMap["patch"] = ver.Patch()
			buildMap["prerelease"] = ver.Prerelease()
		}

		output["version"] = config.CLIVersion

		output["build"] = buildMap

		switch outputFormat {
		case "yaml":
			text, _ := goyaml.Marshal(output)
			fmt.Println(string(text))
		case "json":
			text, _ := json.Marshal(output)
			fmt.Println(string(text))
		default:
			fmt.Println(config.CLIVersion)
		}
	},
}

func init() {
	versionCmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (yaml|json)")
	rootCmd.AddCommand(versionCmd)
}
