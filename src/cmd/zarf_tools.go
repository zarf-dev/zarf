// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/Masterminds/semver/v3"
	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/pkg/oci"
	"github.com/sigstore/cosign/v2/pkg/cosign"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	goyaml "github.com/goccy/go-yaml"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/internal/packager/helm"
	"github.com/zarf-dev/zarf/src/internal/packager/template"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/packager/sources"
	"github.com/zarf-dev/zarf/src/pkg/pki"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
	"github.com/zarf-dev/zarf/src/types"
)

var (
	subAltNames         []string
	outputDirectory     string
	updateCredsInitOpts types.ZarfInitOptions
)

const (
	registryKey     = "registry"
	registryReadKey = "registry-readonly"
	gitKey          = "git"
	gitReadKey      = "git-readonly"
	artifactKey     = "artifact"
	agentKey        = "agent"
)

type getCredsOptions struct {
	outputFormat outputFormat
	outputWriter io.Writer
	cluster      *cluster.Cluster
}

func newGetCredsOptions() *getCredsOptions {
	return &getCredsOptions{
		outputFormat: outputTable,
		// TODO accept output writer as a parameter to the root Zarf command and pass it through here
		outputWriter: message.OutputWriter,
	}
}

func newGetCredsCommand() *cobra.Command {
	o := newGetCredsOptions()

	cmd := &cobra.Command{
		Use:     "get-creds",
		Short:   lang.CmdToolsGetCredsShort,
		Long:    lang.CmdToolsGetCredsLong,
		Example: lang.CmdToolsGetCredsExample,
		Aliases: []string{"gc"},
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			err := o.complete(ctx)
			if err != nil {
				return err
			}
			return o.run(ctx, args)
		},
	}

	cmd.Flags().VarP(&o.outputFormat, "output-format", "o", "Prints the output in the specified format. Valid options: table, json, yaml")

	return cmd
}

func (o *getCredsOptions) complete(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, cluster.DefaultTimeout)
	defer cancel()
	c, err := cluster.NewClusterWithWait(timeoutCtx)
	if err != nil {
		return err
	}
	o.cluster = c
	return nil
}

func (o *getCredsOptions) run(ctx context.Context, args []string) error {
	state, err := o.cluster.LoadState(ctx)
	if err != nil {
		return err
	}
	// TODO: Determine if this is actually needed.
	if state.Distro == "" {
		return errors.New("zarf state secret did not load properly")
	}

	if len(args) > 0 {
		// If a component name is provided, only show that component's credentials
		// Printing both the pterm output and slogger for now
		printComponentCredential(ctx, state, args[0], o.outputWriter)
		message.PrintComponentCredential(state, args[0])
		return nil
	}
	return printCredentialTable(state, o.outputFormat, o.outputWriter)
}

type credentialInfo struct {
	Application string `json:"application"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	Connect     string `json:"connect"`
	GetCredsKey string `json:"getCredsKey"`
}

// TODO Zarf state should be changed to have empty values when a service is not in use
// Once this change is in place, this function should check if the git server, artifact server, or registry server
// information is empty and avoid printing that service if so
func printCredentialTable(state *types.ZarfState, outputFormat outputFormat, out io.Writer) error {
	var credentials []credentialInfo

	if state.RegistryInfo.IsInternal() {
		credentials = append(credentials,
			credentialInfo{
				Application: "Registry",
				Username:    state.RegistryInfo.PushUsername,
				Password:    state.RegistryInfo.PushPassword,
				Connect:     "zarf connect registry",
				GetCredsKey: registryKey,
			},
			credentialInfo{
				Application: "Registry (read-only)",
				Username:    state.RegistryInfo.PullUsername,
				Password:    state.RegistryInfo.PullPassword,
				Connect:     "zarf connect registry",
				GetCredsKey: registryReadKey,
			},
		)
	}

	credentials = append(credentials,
		credentialInfo{
			Application: "Git",
			Username:    state.GitServer.PushUsername,
			Password:    state.GitServer.PushPassword,
			Connect:     "zarf connect git",
			GetCredsKey: gitKey,
		},
		credentialInfo{
			Application: "Git (read-only)",
			Username:    state.GitServer.PullUsername,
			Password:    state.GitServer.PullPassword,
			Connect:     "zarf connect git",
			GetCredsKey: gitReadKey,
		},
		credentialInfo{
			Application: "Artifact Token",
			Username:    state.ArtifactServer.PushUsername,
			Password:    state.ArtifactServer.PushToken,
			Connect:     "zarf connect git",
			GetCredsKey: artifactKey,
		},
	)

	switch outputFormat {
	case outputJSON:
		output, err := json.MarshalIndent(credentials, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(out, string(output))
	case outputYAML:
		output, err := goyaml.Marshal(credentials)
		if err != nil {
			return err
		}
		fmt.Fprint(out, string(output))
	case outputTable:
		header := []string{"Application", "Username", "Password", "Connect", "Get-Creds Key"}
		var tableData [][]string
		for _, cred := range credentials {
			tableData = append(tableData, []string{
				cred.Application, cred.Username, cred.Password, cred.Connect, cred.GetCredsKey,
			})
		}
		message.TableWithWriter(out, header, tableData)
	default:
		return fmt.Errorf("unsupported output format: %s", outputFormat)
	}
	return nil
}

func printComponentCredential(ctx context.Context, state *types.ZarfState, componentName string, out io.Writer) {
	l := logger.From(ctx)
	switch strings.ToLower(componentName) {
	case gitKey:
		l.Info("Git server push password", "username", state.GitServer.PushUsername)
		fmt.Fprintln(out, state.GitServer.PushPassword)
	case gitReadKey:
		l.Info("Git server (read-only) password", "username", state.GitServer.PullUsername)
		fmt.Fprintln(out, state.GitServer.PullPassword)
	case artifactKey:
		l.Info("artifact server token", "username", state.ArtifactServer.PushUsername)
		fmt.Fprintln(out, state.ArtifactServer.PushToken)
	case registryKey:
		l.Info("image registry password", "username", state.RegistryInfo.PushUsername)
		fmt.Fprintln(out, state.RegistryInfo.PushPassword)
	case registryReadKey:
		l.Info("image registry (read-only) password", "username", state.RegistryInfo.PullUsername)
		fmt.Fprintln(out, state.RegistryInfo.PullPassword)
	default:
		l.Warn("unknown component", "component", componentName)
	}
}

type updateCredsOptions struct{}

func newUpdateCredsCommand(v *viper.Viper) *cobra.Command {
	o := updateCredsOptions{}

	cmd := &cobra.Command{
		Use:     "update-creds",
		Short:   lang.CmdToolsUpdateCredsShort,
		Long:    lang.CmdToolsUpdateCredsLong,
		Example: lang.CmdToolsUpdateCredsExample,
		Aliases: []string{"uc"},
		Args:    cobra.MaximumNArgs(1),
		RunE:    o.run,
	}

	// Always require confirm flag (no viper)
	cmd.Flags().BoolVar(&config.CommonOptions.Confirm, "confirm", false, lang.CmdToolsUpdateCredsConfirmFlag)

	// Flags for using an external Git server
	cmd.Flags().StringVar(&updateCredsInitOpts.GitServer.Address, "git-url", v.GetString(VInitGitURL), lang.CmdInitFlagGitURL)
	cmd.Flags().StringVar(&updateCredsInitOpts.GitServer.PushUsername, "git-push-username", v.GetString(VInitGitPushUser), lang.CmdInitFlagGitPushUser)
	cmd.Flags().StringVar(&updateCredsInitOpts.GitServer.PushPassword, "git-push-password", v.GetString(VInitGitPushPass), lang.CmdInitFlagGitPushPass)
	cmd.Flags().StringVar(&updateCredsInitOpts.GitServer.PullUsername, "git-pull-username", v.GetString(VInitGitPullUser), lang.CmdInitFlagGitPullUser)
	cmd.Flags().StringVar(&updateCredsInitOpts.GitServer.PullPassword, "git-pull-password", v.GetString(VInitGitPullPass), lang.CmdInitFlagGitPullPass)

	// Flags for using an external registry
	cmd.Flags().StringVar(&updateCredsInitOpts.RegistryInfo.Address, "registry-url", v.GetString(VInitRegistryURL), lang.CmdInitFlagRegURL)
	cmd.Flags().StringVar(&updateCredsInitOpts.RegistryInfo.PushUsername, "registry-push-username", v.GetString(VInitRegistryPushUser), lang.CmdInitFlagRegPushUser)
	cmd.Flags().StringVar(&updateCredsInitOpts.RegistryInfo.PushPassword, "registry-push-password", v.GetString(VInitRegistryPushPass), lang.CmdInitFlagRegPushPass)
	cmd.Flags().StringVar(&updateCredsInitOpts.RegistryInfo.PullUsername, "registry-pull-username", v.GetString(VInitRegistryPullUser), lang.CmdInitFlagRegPullUser)
	cmd.Flags().StringVar(&updateCredsInitOpts.RegistryInfo.PullPassword, "registry-pull-password", v.GetString(VInitRegistryPullPass), lang.CmdInitFlagRegPullPass)

	// Flags for using an external artifact server
	cmd.Flags().StringVar(&updateCredsInitOpts.ArtifactServer.Address, "artifact-url", v.GetString(VInitArtifactURL), lang.CmdInitFlagArtifactURL)
	cmd.Flags().StringVar(&updateCredsInitOpts.ArtifactServer.PushUsername, "artifact-push-username", v.GetString(VInitArtifactPushUser), lang.CmdInitFlagArtifactPushUser)
	cmd.Flags().StringVar(&updateCredsInitOpts.ArtifactServer.PushToken, "artifact-push-token", v.GetString(VInitArtifactPushToken), lang.CmdInitFlagArtifactPushToken)

	cmd.Flags().SortFlags = true

	return cmd
}

func (o *updateCredsOptions) run(cmd *cobra.Command, args []string) error {
	validKeys := []string{message.RegistryKey, message.GitKey, message.ArtifactKey, message.AgentKey}
	if len(args) == 0 {
		args = validKeys
	} else {
		if !slices.Contains(validKeys, args[0]) {
			cmd.Help()
			return fmt.Errorf("invalid service key specified, valid key choices are: %v", validKeys)
		}
	}

	ctx := cmd.Context()
	l := logger.From(ctx)

	timeoutCtx, cancel := context.WithTimeout(ctx, cluster.DefaultTimeout)
	defer cancel()
	c, err := cluster.NewClusterWithWait(timeoutCtx)
	if err != nil {
		return err
	}

	oldState, err := c.LoadState(ctx)
	if err != nil {
		return err
	}
	// TODO: Determine if this is actually needed.
	if oldState.Distro == "" {
		return errors.New("zarf state secret did not load properly")
	}
	newState, err := state.MergeZarfState(oldState, updateCredsInitOpts, args)
	if err != nil {
		return fmt.Errorf("unable to update Zarf credentials: %w", err)
	}

	// Printing both the pterm output and slogger for now
	message.PrintCredentialUpdates(oldState, newState, args)
	printCredentialUpdates(ctx, oldState, newState, args)

	confirm := config.CommonOptions.Confirm

	if !confirm {
		prompt := &survey.Confirm{
			Message: lang.CmdToolsUpdateCredsConfirmContinue,
		}
		if err := survey.AskOne(prompt, &confirm); err != nil {
			return fmt.Errorf("confirm selection canceled: %w", err)
		}
	}

	if !confirm {
		return nil
	}

	// Update registry and git pull secrets
	if slices.Contains(args, message.RegistryKey) {
		err := c.UpdateZarfManagedImageSecrets(ctx, newState)
		if err != nil {
			return err
		}
	}
	if slices.Contains(args, message.GitKey) {
		err := c.UpdateZarfManagedGitSecrets(ctx, newState)
		if err != nil {
			return err
		}
	}
	// TODO once Zarf is changed so the default state is empty for a service when it is not deployed
	// and sufficient time has passed for users state to get updated we can remove this check
	internalGitServerExists, err := c.InternalGitServerExists(cmd.Context())
	if err != nil {
		return err
	}

	// Update artifact token (if internal)
	if slices.Contains(args, message.ArtifactKey) && newState.ArtifactServer.PushToken == "" && newState.ArtifactServer.IsInternal() && internalGitServerExists {
		newState.ArtifactServer.PushToken, err = c.UpdateInternalArtifactServerToken(ctx, oldState.GitServer)
		if err != nil {
			return fmt.Errorf("unable to create the new Gitea artifact token: %w", err)
		}
	}

	// Save the final Zarf State
	err = c.SaveZarfState(ctx, newState)
	if err != nil {
		return fmt.Errorf("failed to save the Zarf State to the cluster: %w", err)
	}

	helmOpts := helm.InstallUpgradeOpts{
		VariableConfig: template.GetZarfVariableConfig(cmd.Context()),
		State:          newState,
		Cluster:        c,
		AirgapMode:     true,
		Timeout:        config.ZarfDefaultTimeout,
		Retries:        config.ZarfDefaultRetries,
	}

	// Update Zarf 'init' component Helm releases if present
	if slices.Contains(args, message.RegistryKey) && newState.RegistryInfo.IsInternal() {
		err = helm.UpdateZarfRegistryValues(ctx, helmOpts)
		if err != nil {
			// Warn if we couldn't actually update the registry (it might not be installed and we should try to continue)
			message.Warnf(lang.CmdToolsUpdateCredsUnableUpdateRegistry, err.Error())
			l.Warn("unable to update Zarf Registry values", "error", err.Error())
		}
	}
	if slices.Contains(args, message.GitKey) && newState.GitServer.IsInternal() && internalGitServerExists {
		err := c.UpdateInternalGitServerSecret(cmd.Context(), oldState.GitServer, newState.GitServer)
		if err != nil {
			return fmt.Errorf("unable to update Zarf Git Server values: %w", err)
		}
	}
	if slices.Contains(args, message.AgentKey) {
		err = helm.UpdateZarfAgentValues(ctx, helmOpts)
		if err != nil {
			// Warn if we couldn't actually update the agent (it might not be installed and we should try to continue)
			message.Warnf(lang.CmdToolsUpdateCredsUnableUpdateAgent, err.Error())
			l.Warn("unable to update Zarf Agent TLS secrets", "error", err.Error())
		}
	}

	return nil
}

func printCredentialUpdates(ctx context.Context, oldState *types.ZarfState, newState *types.ZarfState, services []string) {
	// Pause the logfile's output to avoid credentials being printed to the log file
	l := logger.From(ctx)
	l.Info("--- printing credential updates. Sensitive values will be redacted ---")
	for _, service := range services {
		switch service {
		case registryKey:
			oR := oldState.RegistryInfo
			nR := newState.RegistryInfo
			l.Info("registry URL address", "existing", oR.Address, "replacement", nR.Address)
			l.Info("registry push username", "existing", oR.PushUsername, "replacement", nR.PushUsername)
			l.Info("registry push password", "changed", oR.PushPassword != nR.PushPassword)
			l.Info("registry pull username", "existing", oR.PullUsername, "replacement", nR.PullUsername)
			l.Info("registry pull password", "changed", oR.PullPassword != nR.PullPassword)
		case gitKey:
			oG := oldState.GitServer
			nG := newState.GitServer
			l.Info("Git server URL address", "existing", oG.Address, "replacement", nG.Address)
			l.Info("Git server push username", "existing", oG.PushUsername, "replacement", nG.PushUsername)
			l.Info("Git server push password", "changed", oG.PushPassword != nG.PushPassword)
			l.Info("Git server pull username", "existing", oG.PullUsername, "replacement", nG.PullUsername)
			l.Info("Git server pull password", "changed", oG.PullPassword != nG.PullPassword)
		case artifactKey:
			oA := oldState.ArtifactServer
			nA := newState.ArtifactServer
			l.Info("artifact server URL address", "existing", oA.Address, "replacement", nA.Address)
			l.Info("artifact server push username", "existing", oA.PushUsername, "replacement", nA.PushUsername)
			l.Info("artifact server push token", "changed", oA.PushToken != nA.PushToken)
		case agentKey:
			oT := oldState.AgentTLS
			nT := newState.AgentTLS
			l.Info("agent certificate authority", "changed", string(oT.CA) != string(nT.CA))
			l.Info("agent public certificate", "changed", string(oT.Cert) != string(nT.Cert))
			l.Info("agent private key", "changed", string(oT.Key) != string(nT.Key))
		}
	}
}

type clearCacheOptions struct{}

func newClearCacheCommand() *cobra.Command {
	o := &clearCacheOptions{}

	cmd := &cobra.Command{
		Use:     "clear-cache",
		Aliases: []string{"c"},
		Short:   lang.CmdToolsClearCacheShort,
		RunE:    o.run,
	}

	cmd.Flags().StringVar(&config.CommonOptions.CachePath, "zarf-cache", config.ZarfDefaultCachePath, lang.CmdToolsClearCacheFlagCachePath)

	return cmd
}

func (o *clearCacheOptions) run(cmd *cobra.Command, _ []string) error {
	l := logger.From(cmd.Context())
	cachePath, err := config.GetAbsCachePath()
	if err != nil {
		return err
	}
	message.Notef(lang.CmdToolsClearCacheDir, cachePath)
	l.Info("clearing cache", "path", cachePath)
	if err := os.RemoveAll(cachePath); err != nil {
		return fmt.Errorf("unable to clear the cache directory %s: %w", cachePath, err)
	}
	message.Successf(lang.CmdToolsClearCacheSuccess, cachePath)

	return nil
}

type downloadInitOptions struct {
	version string
}

func newDownloadInitCommand() *cobra.Command {
	o := &downloadInitOptions{}

	cmd := &cobra.Command{
		Use:   "download-init",
		Short: lang.CmdToolsDownloadInitShort,
		RunE:  o.run,
	}

	cmd.Flags().StringVarP(&outputDirectory, "output-directory", "o", "", lang.CmdToolsDownloadInitFlagOutputDirectory)
	cmd.Flags().StringVarP(&o.version, "version", "v", o.version, "Specify version to download (defaults to current CLI version)")
	return cmd
}

func (o *downloadInitOptions) run(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	var url string

	if o.version == "" {
		url = zoci.GetInitPackageURL(config.CLIVersion)
	} else {
		ver, err := semver.NewVersion(o.version)
		if err != nil {
			return fmt.Errorf("unable to parse version %s: %w", o.version, err)
		}

		url = zoci.GetInitPackageURL(fmt.Sprintf("v%s", ver.String()))
	}
	remote, err := zoci.NewRemote(ctx, url, oci.PlatformForArch(config.GetArch()))
	if err != nil {
		return fmt.Errorf("unable to download the init package: %w", err)
	}
	source := &sources.OCISource{Remote: remote}
	_, err = source.Collect(ctx, outputDirectory)
	if err != nil {
		return fmt.Errorf("unable to download the init package: %w", err)
	}
	return nil
}

type genPKIOptions struct{}

func newGenPKICommand() *cobra.Command {
	o := &genPKIOptions{}

	cmd := &cobra.Command{
		Use:     "gen-pki HOST",
		Aliases: []string{"pki"},
		Short:   lang.CmdToolsGenPkiShort,
		Args:    cobra.ExactArgs(1),
		RunE:    o.run,
	}

	cmd.Flags().StringArrayVar(&subAltNames, "sub-alt-name", []string{}, lang.CmdToolsGenPkiFlagAltName)

	return cmd
}

func (o *genPKIOptions) run(cmd *cobra.Command, args []string) error {
	pki, err := pki.GeneratePKI(args[0], subAltNames...)
	if err != nil {
		return err
	}
	if err := os.WriteFile("tls.ca", pki.CA, helpers.ReadAllWriteUser); err != nil {
		return err
	}
	if err := os.WriteFile("tls.crt", pki.Cert, helpers.ReadAllWriteUser); err != nil {
		return err
	}
	if err := os.WriteFile("tls.key", pki.Key, helpers.ReadWriteUser); err != nil {
		return err
	}
	message.Successf(lang.CmdToolsGenPkiSuccess, args[0])
	logger.From(cmd.Context()).Info("successfully created a chain of trust", "host", args[0])

	return nil
}

type genKeyOptions struct{}

func newGenKeyCommand() *cobra.Command {
	o := &genKeyOptions{}

	cmd := &cobra.Command{
		Use:     "gen-key",
		Aliases: []string{"key"},
		Short:   lang.CmdToolsGenKeyShort,
		RunE:    o.run,
	}

	return cmd
}

func (o *genKeyOptions) run(cmd *cobra.Command, _ []string) error {
	// Utility function to prompt the user for the password to the private key
	passwordFunc := func(bool) ([]byte, error) {
		// perform the first prompt
		var password string
		prompt := &survey.Password{
			Message: lang.CmdToolsGenKeyPrompt,
		}
		if err := survey.AskOne(prompt, &password); err != nil {
			return nil, fmt.Errorf(lang.CmdToolsGenKeyErrUnableGetPassword, err.Error())
		}

		// perform the second prompt
		var doubleCheck string
		rePrompt := &survey.Password{
			Message: lang.CmdToolsGenKeyPromptAgain,
		}
		if err := survey.AskOne(rePrompt, &doubleCheck); err != nil {
			return nil, fmt.Errorf(lang.CmdToolsGenKeyErrUnableGetPassword, err.Error())
		}

		// check if the passwords match
		if password != doubleCheck {
			return nil, fmt.Errorf(lang.CmdToolsGenKeyErrPasswordsNotMatch)
		}

		return []byte(password), nil
	}

	// Use cosign to generate the keypair
	keyBytes, err := cosign.GenerateKeyPair(passwordFunc)
	if err != nil {
		return fmt.Errorf("unable to generate key pair: %w", err)
	}

	prvKeyFileName := "cosign.key"
	pubKeyFileName := "cosign.pub"

	// Check if we are about to overwrite existing key files
	_, prvKeyExistsErr := os.Stat(prvKeyFileName)
	_, pubKeyExistsErr := os.Stat(pubKeyFileName)
	if prvKeyExistsErr == nil || pubKeyExistsErr == nil {
		var confirm bool
		confirmOverwritePrompt := &survey.Confirm{
			Message: fmt.Sprintf(lang.CmdToolsGenKeyPromptExists, prvKeyFileName),
		}
		err := survey.AskOne(confirmOverwritePrompt, &confirm)
		if err != nil {
			return err
		}
		if !confirm {
			return errors.New("did not receive confirmation for overwriting key file(s)")
		}
	}

	// Write the key file contents to disk
	if err := os.WriteFile(prvKeyFileName, keyBytes.PrivateBytes, helpers.ReadWriteUser); err != nil {
		return err
	}
	if err := os.WriteFile(pubKeyFileName, keyBytes.PublicBytes, helpers.ReadAllWriteUser); err != nil {
		return err
	}

	message.Successf(lang.CmdToolsGenKeySuccess, prvKeyFileName, pubKeyFileName)
	logger.From(cmd.Context()).Info("Successfully generated key pair",
		"private-key-path", prvKeyExistsErr,
		"public-key-path", pubKeyFileName)

	return nil
}
