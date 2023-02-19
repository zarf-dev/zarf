// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/anchore/syft/cmd/syft/cli"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/pki"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/exec"
	k9s "github.com/derailed/k9s/cmd"
	craneCmd "github.com/google/go-containerregistry/cmd/crane/cmd"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/mholt/archiver/v3"
	"github.com/spf13/cobra"
	kubeCLI "k8s.io/component-base/cli"
	kubeCmd "k8s.io/kubectl/pkg/cmd"

	// Import to initialize client auth plugins.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var (
	subAltNames []string
	waitTimeout string
)

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

var waitForCmd = &cobra.Command{
	Use:     "wait-for {NAMESPACE} {RESOURCE} {NAME} {CONDITION}",
	Aliases: []string{"w", "wait"},
	Short:   lang.CmdToolsWaitForShort,
	Long:    lang.CmdToolsWaitForLong,
	Example: `zarf tools wait-for default pod my-pod-name ready`,
	Args:    cobra.ExactArgs(4),
	Run: func(cmd *cobra.Command, args []string) {
		// Parse the timeout string
		timeout, err := time.ParseDuration(waitTimeout)
		if err != nil {
			message.Fatalf(err, lang.CmdToolsWaitForErrTimeoutString, waitTimeout)
		}

		namespace, resource, name, condition := args[0], args[1], args[2], args[3]
		zarf, err := utils.GetFinalExecutablePath()
		if err != nil {
			message.Fatal(err, lang.CmdToolsWaitForErrZarfPath)
		}

		expired := time.After(timeout)

		conditionMsg := fmt.Sprintf("Waiting for %s/%s in namespace %s to be %s.", resource, name, namespace, condition)
		existMsg := fmt.Sprintf("Waiting for %s/%s in namespace %s to exist.", resource, name, namespace)
		spinner := message.NewProgressSpinner(existMsg)
		defer spinner.Stop()

		for {
			// Delay the check for 1 second
			time.Sleep(time.Second)

			select {
			case <-expired:
				message.Fatal(nil, lang.CmdToolsWaitForErrTimeout)

			default:
				spinner.Updatef(existMsg)
				// Check if the resource exists.
				args := []string{"tools", "kubectl", "get", "-n", namespace, resource, name}
				if stdout, stderr, err := exec.Cmd(zarf, args...); err != nil {
					message.Debug(stdout, stderr, err)
					continue
				}

				spinner.Updatef(conditionMsg)
				// Wait for the resource to meet the given condition.
				args = []string{"tools", "kubectl", "wait", "-n", namespace, resource, name, "--for", "condition=" + condition, "--timeout=" + waitTimeout}
				if stdout, stderr, err := exec.Cmd(zarf, args...); err != nil {
					message.Debug(stdout, stderr, err)
					continue
				}

				spinner.Successf(conditionMsg)
				os.Exit(0)
			}
		}
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

	// Kubectl stub command.
	kubectlCmd := &cobra.Command{
		Short: lang.CmdToolsKubectlDocs,
		Run:   func(cmd *cobra.Command, args []string) {},
	}

	// Only load this command if it is being called directly.
	if isVendorCmd([]string{"kubectl", "k"}) {
		// Add the kubectl command to the tools command.
		kubectlCmd = kubeCmd.NewDefaultKubectlCommand()

		if err := kubeCLI.RunNoErrOutput(kubectlCmd); err != nil {
			// @todo(jeff-mccoy) - Kubectl gets mad about being a subcommand.
			message.Debug(err)
		}
	}

	kubectlCmd.Use = "kubectl"
	kubectlCmd.Aliases = []string{"k"}

	toolsCmd.AddCommand(kubectlCmd)

	toolsCmd.AddCommand(waitForCmd)
	waitForCmd.Flags().StringVar(&waitTimeout, "timeout", "5m", lang.CmdToolsWaitForFlagTimeout)
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

// isVendorCmd checks if the command is a vendor command.
func isVendorCmd(cmd []string) bool {
	a := os.Args
	if len(a) > 2 {
		if a[1] == "tools" || a[1] == "t" {
			if utils.SliceContains(cmd, a[2]) {
				return true
			}
		}
	}

	return false
}

// Check if the command is being run as a vendor-only command
func checkVendorOnly() bool {
	vendorCmd := []string{
		"kubectl",
		"k",
		"syft",
		"sbom",
		"s",
		"k9s",
		"monitor",
		"wait-for",
		"wait",
		"w",
	}

	// Check for "zarf tools|t <cmd>" where <cmd> is in the vendorCmd list
	return isVendorCmd(vendorCmd)
}
