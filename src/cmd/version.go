// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"fmt"
	"os"
	"runtime"

	"github.com/Masterminds/semver/v3"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/spf13/cobra"

	"runtime/debug"

	goyaml "github.com/goccy/go-yaml"
)

var showDependencies bool
var showBuild bool

var versionCmd = &cobra.Command{
	Use:     "version",
	Aliases: []string{"v"},
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		config.SkipLogFile = true
	},
	Short: lang.CmdVersionShort,
	Long:  lang.CmdVersionLong,
	Run: func(cmd *cobra.Command, args []string) {
		yamlOutput := make(map[string]interface{})
		if showDependencies || showBuild {
			buildInfo, ok := debug.ReadBuildInfo()
			if !ok {
				fmt.Println()
				fmt.Println("Failed to get build info")
				return
			}
			if showDependencies {
				depMap := map[string]string{}
				for _, dep := range buildInfo.Deps {
					if dep.Replace != nil {
						depMap[dep.Path] = fmt.Sprintf("-> %s %s", dep.Replace.Path, dep.Replace.Version)
					} else {
						depMap[dep.Path] = dep.Version
					}
				}
				yamlOutput["dependencies"] = depMap
			}
			if showBuild {
				buildMap := make(map[string]interface{})
				buildMap["platform"] = runtime.GOOS + "/" + runtime.GOARCH
				buildMap["goVersion"] = runtime.Version()
				ver := semver.MustParse(config.CLIVersion)
				buildMap["major"] = ver.Major()
				buildMap["minor"] = ver.Minor()
				buildMap["patch"] = ver.Patch()
				buildMap["prerelease"] = ver.Prerelease()

				yamlOutput["build"] = buildMap
			}

			text, _ := goyaml.Marshal(yamlOutput)
			fmt.Println(string(text))
		} else {
			fmt.Println(config.CLIVersion)
		}
	},
}

func isVersionCmd() bool {
	args := os.Args
	return len(args) > 1 && (args[1] == "version" || args[1] == "v")
}

func init() {
	versionCmd.Flags().BoolVar(&showDependencies, "dependencies", false, "Show binary's dependencies")
	versionCmd.Flags().BoolVar(&showBuild, "build", false, "Show binary's build settings")
	rootCmd.AddCommand(versionCmd)
}
