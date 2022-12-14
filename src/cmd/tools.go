// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for zarf
package cmd

import (
	"fmt"
	"os"

	"github.com/anchore/syft/cmd/syft/cli"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/pki"
	k9s "github.com/derailed/k9s/cmd"
	craneCmd "github.com/google/go-containerregistry/cmd/crane/cmd"
	"github.com/mholt/archiver/v3"
	"github.com/spf13/cobra"
)

var subAltNames []string

var toolsCmd = &cobra.Command{
	Use:     "tools",
	Aliases: []string{"t"},
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		skipLogFile = true
		cliSetup()
	},
	Short: lang.CmdToolsShort,
}

// destroyCmd represents the init command
var archiverCmd = &cobra.Command{
	Use:     "archiver",
	Aliases: []string{"a"},
	Short:   lang.CmdToolsArchiverShort,
}

var archiverCompressCmd = &cobra.Command{
	Use:     "compress {SOURCES} {ARCHIVE}",
	Aliases: []string{"c"},
	Short:   lang.CmdToolsArchiverCompressShort,
	Args:    cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		sourceFiles, destinationArchive := args[:len(args)-1], args[len(args)-1]
		err := archiver.Archive(sourceFiles, destinationArchive)
		if err != nil {
			message.Fatal(err, lang.CmdToolsArchiverCompressErr)
		}
	},
}

var archiverDecompressCmd = &cobra.Command{
	Use:     "decompress {ARCHIVE} {DESTINATION}",
	Aliases: []string{"d"},
	Short:   lang.CmdToolsArchiverDecompressShort,
	Args:    cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		sourceArchive, destinationPath := args[0], args[1]
		err := archiver.Unarchive(sourceArchive, destinationPath)
		if err != nil {
			message.Fatal(err, lang.CmdToolsArchiverDecompressErr)
		}
	},
}

var registryCmd = &cobra.Command{
	Use:     "registry",
	Aliases: []string{"r", "crane"},
	Short:   lang.CmdToolsRegistryShort,
}

var readCredsCmd = &cobra.Command{
	Use:   "get-git-password",
	Short: lang.CmdToolsGetGitPasswdShort,
	Long:  lang.CmdToolsGetGitPasswdLong,
	Run: func(cmd *cobra.Command, args []string) {
		state, err := cluster.NewClusterOrDie().LoadZarfState()
		if err != nil || state.Distro == "" {
			// If no distro the zarf secret did not load properly
			message.Fatalf(nil, lang.ErrLoadState)
		}

		message.Note(lang.CmdToolsGetGitPasswdInfo)
		fmt.Println(state.GitServer.PushPassword)
	},
}

var k9sCmd = &cobra.Command{
	Use:     "monitor",
	Aliases: []string{"m", "k9s"},
	Short:   lang.CmdToolsMonitorShort,
	Run: func(cmd *cobra.Command, args []string) {
		// Hack to make k9s think it's all alone
		os.Args = []string{os.Args[0], "-n", "zarf"}
		k9s.Execute()
	},
}

var clearCacheCmd = &cobra.Command{
	Use:     "clear-cache",
	Aliases: []string{"c"},
	Short:   lang.CmdToolsClearCacheShort,
	Run: func(cmd *cobra.Command, args []string) {
		message.Debugf("Cache directory set to: %s", config.GetAbsCachePath())
		if err := os.RemoveAll(config.GetAbsCachePath()); err != nil {
			message.Fatalf(err, lang.CmdToolsClearCacheErr, config.GetAbsCachePath())
		}
		message.SuccessF(lang.CmdToolsClearCacheSuccess, config.GetAbsCachePath())
	},
}

var generatePKICmd = &cobra.Command{
	Use:     "gen-pki {HOST}",
	Aliases: []string{"pki"},
	Short:   lang.CmdToolsGenPkiShort,
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		pki := pki.GeneratePKI(args[0], subAltNames...)
		if err := os.WriteFile("tls.ca", pki.CA, 0644); err != nil {
			message.Fatalf(err, lang.ErrWritingFile, "tls.ca", err.Error())
		}
		if err := os.WriteFile("tls.crt", pki.Cert, 0644); err != nil {
			message.Fatalf(err, lang.ErrWritingFile, "tls.crt", err.Error())
		}
		if err := os.WriteFile("tls.key", pki.Key, 0600); err != nil {
			message.Fatalf(err, lang.ErrWritingFile, "tls.key", err.Error())
		}
		message.SuccessF(lang.CmdToolsGenPkiSuccess, args[0])
	},
}

func init() {
	rootCmd.AddCommand(toolsCmd)
	toolsCmd.AddCommand(archiverCmd)
	toolsCmd.AddCommand(readCredsCmd)
	toolsCmd.AddCommand(k9sCmd)
	toolsCmd.AddCommand(registryCmd)

	toolsCmd.AddCommand(clearCacheCmd)
	clearCacheCmd.Flags().StringVar(&config.CommonOptions.CachePath, "zarf-cache", config.ZarfDefaultCachePath, lang.CmdToolsClearCacheFlagCachePath)

	toolsCmd.AddCommand(generatePKICmd)
	generatePKICmd.Flags().StringArrayVar(&subAltNames, "sub-alt-name", []string{}, lang.CmdToolsGenPkiFlagAltName)

	archiverCmd.AddCommand(archiverCompressCmd)
	archiverCmd.AddCommand(archiverDecompressCmd)

	cranePlatformOptions := config.GetCraneOptions(false)

	craneLogin := craneCmd.NewCmdAuthLogin()
	craneLogin.Example = ""

	craneCatalog := craneCmd.NewCmdCatalog(&cranePlatformOptions)
	craneCatalog.Example = ""

	registryCmd.AddCommand(craneLogin)
	registryCmd.AddCommand(craneCmd.NewCmdPull(&cranePlatformOptions))
	registryCmd.AddCommand(craneCmd.NewCmdPush(&cranePlatformOptions))
	registryCmd.AddCommand(craneCmd.NewCmdCopy(&cranePlatformOptions))
	registryCmd.AddCommand(craneCatalog)

	syftCmd, err := cli.New()
	if err != nil {
		message.Fatal(err, lang.CmdToolsSbomErr)
	}
	syftCmd.Use = "sbom"
	syftCmd.Short = lang.CmdToolsSbomShort
	syftCmd.Aliases = []string{"s", "syft"}
	syftCmd.Example = ""

	for _, subCmd := range syftCmd.Commands() {
		subCmd.Example = ""
	}

	toolsCmd.AddCommand(syftCmd)
}
