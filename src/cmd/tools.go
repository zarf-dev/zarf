// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/anchore/syft/cmd/syft/cli"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/pki"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	k9s "github.com/derailed/k9s/cmd"
	craneCmd "github.com/google/go-containerregistry/cmd/crane/cmd"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/mholt/archiver/v3"
	"github.com/spf13/cobra"
)

var subAltNames []string
var decompressLayers bool

var toolsCmd = &cobra.Command{
	Use:     "tools",
	Aliases: []string{"t"},
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		skipLogFile = true
		cliSetup()
	},
	Short: lang.CmdToolsShort,
}

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

		// Decompress component layers in the destination path
		if decompressLayers {
			layersDir := filepath.Join(destinationPath, "components")

			files, err := os.ReadDir(layersDir)
			if err != nil {
				message.Fatalf(err, "failed to read the layers of components")
			}
			for _, file := range files {
				if strings.HasSuffix(file.Name(), "tar.zst") {
					if err := archiver.Unarchive(filepath.Join(layersDir, file.Name()), layersDir); err != nil {
						message.Fatalf(err, "failed to decompress the component layer")
					} else {
						// Without unarchive error, delete original tar.zst in component folder
						// This will leave the tar.zst if their is a failure for post mortem check 
						_ = os.Remove(filepath.Join(layersDir, file.Name()))
					}
				}
			}
		}
	},
}

var registryCmd = &cobra.Command{
	Use:     "registry",
	Aliases: []string{"r", "crane"},
	Short:   lang.CmdToolsRegistryShort,
}

var readCredsCmd = &cobra.Command{
	Use:    "get-git-password",
	Hidden: true,
	Short:  lang.CmdToolsGetGitPasswdShort,
	Long:   lang.CmdToolsGetGitPasswdLong,
	Run: func(cmd *cobra.Command, args []string) {
		state, err := cluster.NewClusterOrDie().LoadZarfState()
		if err != nil || state.Distro == "" {
			// If no distro the zarf secret did not load properly
			message.Fatalf(nil, lang.ErrLoadState)
		}

		message.Note(lang.CmdToolsGetGitPasswdInfo)
		message.Warn(lang.CmdToolGetGitDeprecation)
		utils.PrintComponentCredential(state, "git")
	},
}

var readAllCredsCmd = &cobra.Command{
	Use:     "get-creds",
	Short:   lang.CmdToolsGetCredsShort,
	Long:    lang.CmdToolsGetCredsLong,
	Aliases: []string{"gc"},
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		state, err := cluster.NewClusterOrDie().LoadZarfState()
		if err != nil || state.Distro == "" {
			// If no distro the zarf secret did not load properly
			message.Fatalf(nil, lang.ErrLoadState)
		}

		if len(args) > 0 {
			// If a component name is provided, only show that component's credentials
			utils.PrintComponentCredential(state, args[0])
		} else {
			utils.PrintCredentialTable(state, nil)
		}
	},
}

var k9sCmd = &cobra.Command{
	Use:     "monitor",
	Aliases: []string{"m", "k9s"},
	Short:   lang.CmdToolsMonitorShort,
	Run: func(cmd *cobra.Command, args []string) {
		// Hack to make k9s think it's all alone
		os.Args = []string{os.Args[0]}
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
	toolsCmd.AddCommand(readAllCredsCmd)

	toolsCmd.AddCommand(clearCacheCmd)
	clearCacheCmd.Flags().StringVar(&config.CommonOptions.CachePath, "zarf-cache", config.ZarfDefaultCachePath, lang.CmdToolsClearCacheFlagCachePath)

	toolsCmd.AddCommand(generatePKICmd)
	generatePKICmd.Flags().StringArrayVar(&subAltNames, "sub-alt-name", []string{}, lang.CmdToolsGenPkiFlagAltName)

	archiverCmd.AddCommand(archiverCompressCmd)
	archiverCmd.AddCommand(archiverDecompressCmd)
	archiverDecompressCmd.Flags().BoolVar(&decompressLayers, "decompress-all", false, "Decompress all layers in the archive")

	cranePlatformOptions := config.GetCraneOptions(false)

	craneLogin := craneCmd.NewCmdAuthLogin()
	craneLogin.Example = ""

	registryCmd.AddCommand(craneLogin)
	registryCmd.AddCommand(craneCmd.NewCmdPull(&cranePlatformOptions))
	registryCmd.AddCommand(craneCmd.NewCmdPush(&cranePlatformOptions))
	registryCmd.AddCommand(craneCmd.NewCmdCopy(&cranePlatformOptions))
	registryCmd.AddCommand(zarfCraneCatalog(&cranePlatformOptions))

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

// Wrap the original crane catalog with a zarf specific version
func zarfCraneCatalog(cranePlatformOptions *[]crane.Option) *cobra.Command {
	craneCatalog := craneCmd.NewCmdCatalog(cranePlatformOptions)

	eg := `  # list the repos internal to Zarf
  $ zarf tools registry catalog

  # list the repos for reg.example.com
  $ zarf tools registry catalog reg.example.com`

	craneCatalog.Example = eg
	craneCatalog.Args = nil

	originalCatalogFn := craneCatalog.RunE

	craneCatalog.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return originalCatalogFn(cmd, args)
		}

		// Load Zarf state
		zarfState, err := cluster.NewClusterOrDie().LoadZarfState()
		if err != nil {
			return err
		}

		// Open a tunnel to the Zarf registry
		tunnelReg, err := cluster.NewZarfTunnel()
		if err != nil {
			return err
		}
		tunnelReg.Connect(cluster.ZarfRegistry, false)

		// Add the correct authentication to the crane command options
		authOption := config.GetCraneAuthOption(zarfState.RegistryInfo.PullUsername, zarfState.RegistryInfo.PullPassword)
		*cranePlatformOptions = append(*cranePlatformOptions, authOption)
		registryEndpoint := tunnelReg.Endpoint()

		return originalCatalogFn(cmd, []string{registryEndpoint})
	}

	return craneCatalog
}
