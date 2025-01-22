// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package tools contains the CLI commands for Zarf.
package tools

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/pkg/oci"
	"github.com/sigstore/cosign/v2/pkg/cosign"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/zarf-dev/zarf/src/cmd/common"
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

var subAltNames []string
var outputDirectory string
var updateCredsInitOpts types.ZarfInitOptions

const (
	registryKey     = "registry"
	registryReadKey = "registry-readonly"
	gitKey          = "git"
	gitReadKey      = "git-readonly"
	artifactKey     = "artifact"
	agentKey        = "agent"
)

// GetCredsOptions holds the command-line options for 'tools get-creds' sub-command.
type GetCredsOptions struct{}

// NewGetCredsCommand creates the `tools get-creds` sub-command.
func NewGetCredsCommand() *cobra.Command {
	o := GetCredsOptions{}

	cmd := &cobra.Command{
		Use:     "get-creds",
		Short:   lang.CmdToolsGetCredsShort,
		Long:    lang.CmdToolsGetCredsLong,
		Example: lang.CmdToolsGetCredsExample,
		Aliases: []string{"gc"},
		Args:    cobra.MaximumNArgs(1),
		RunE:    o.run,
	}

	return cmd
}

// Run performs the execution of 'tools get-creds' sub-command.
func (o *GetCredsOptions) run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	timeoutCtx, cancel := context.WithTimeout(ctx, cluster.DefaultTimeout)
	defer cancel()
	c, err := cluster.NewClusterWithWait(timeoutCtx)
	if err != nil {
		return err
	}

	state, err := c.LoadZarfState(ctx)
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
		printComponentCredential(ctx, state, args[0])
		message.PrintComponentCredential(state, args[0])
	} else {
		message.PrintCredentialTable(state, nil)
	}
	return nil
}

func printComponentCredential(ctx context.Context, state *types.ZarfState, componentName string) {
	// TODO (@austinabro321) when we move over to the new logger, we can should add fmt.Println calls
	// to this function as they will be removed from message.PrintComponentCredential
	l := logger.From(ctx)
	switch strings.ToLower(componentName) {
	case gitKey:
		l.Info("Git server push password", "username", state.GitServer.PushUsername)
	case gitReadKey:
		l.Info("Git server (read-only) password", "username", state.GitServer.PullUsername)
	case artifactKey:
		l.Info("artifact server token", "username", state.ArtifactServer.PushUsername)
	case registryKey:
		l.Info("image registry password", "username", state.RegistryInfo.PushUsername)
	case registryReadKey:
		l.Info("image registry (read-only) password", "username", state.RegistryInfo.PullUsername)
	default:
		l.Warn("unknown component", "component", componentName)
	}
}

// UpdateCredsOptions holds the command-line options for 'tools update-creds' sub-command.
type UpdateCredsOptions struct{}

// NewUpdateCredsCommand creates the `tools update-creds` sub-command.
func NewUpdateCredsCommand(v *viper.Viper) *cobra.Command {
	o := UpdateCredsOptions{}

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
	cmd.Flags().StringVar(&updateCredsInitOpts.GitServer.Address, "git-url", v.GetString(common.VInitGitURL), lang.CmdInitFlagGitURL)
	cmd.Flags().StringVar(&updateCredsInitOpts.GitServer.PushUsername, "git-push-username", v.GetString(common.VInitGitPushUser), lang.CmdInitFlagGitPushUser)
	cmd.Flags().StringVar(&updateCredsInitOpts.GitServer.PushPassword, "git-push-password", v.GetString(common.VInitGitPushPass), lang.CmdInitFlagGitPushPass)
	cmd.Flags().StringVar(&updateCredsInitOpts.GitServer.PullUsername, "git-pull-username", v.GetString(common.VInitGitPullUser), lang.CmdInitFlagGitPullUser)
	cmd.Flags().StringVar(&updateCredsInitOpts.GitServer.PullPassword, "git-pull-password", v.GetString(common.VInitGitPullPass), lang.CmdInitFlagGitPullPass)

	// Flags for using an external registry
	cmd.Flags().StringVar(&updateCredsInitOpts.RegistryInfo.Address, "registry-url", v.GetString(common.VInitRegistryURL), lang.CmdInitFlagRegURL)
	cmd.Flags().StringVar(&updateCredsInitOpts.RegistryInfo.PushUsername, "registry-push-username", v.GetString(common.VInitRegistryPushUser), lang.CmdInitFlagRegPushUser)
	cmd.Flags().StringVar(&updateCredsInitOpts.RegistryInfo.PushPassword, "registry-push-password", v.GetString(common.VInitRegistryPushPass), lang.CmdInitFlagRegPushPass)
	cmd.Flags().StringVar(&updateCredsInitOpts.RegistryInfo.PullUsername, "registry-pull-username", v.GetString(common.VInitRegistryPullUser), lang.CmdInitFlagRegPullUser)
	cmd.Flags().StringVar(&updateCredsInitOpts.RegistryInfo.PullPassword, "registry-pull-password", v.GetString(common.VInitRegistryPullPass), lang.CmdInitFlagRegPullPass)

	// Flags for using an external artifact server
	cmd.Flags().StringVar(&updateCredsInitOpts.ArtifactServer.Address, "artifact-url", v.GetString(common.VInitArtifactURL), lang.CmdInitFlagArtifactURL)
	cmd.Flags().StringVar(&updateCredsInitOpts.ArtifactServer.PushUsername, "artifact-push-username", v.GetString(common.VInitArtifactPushUser), lang.CmdInitFlagArtifactPushUser)
	cmd.Flags().StringVar(&updateCredsInitOpts.ArtifactServer.PushToken, "artifact-push-token", v.GetString(common.VInitArtifactPushToken), lang.CmdInitFlagArtifactPushToken)

	cmd.Flags().SortFlags = true

	return cmd
}

// Run performs the execution of 'tools update-creds' sub-command.
func (o *UpdateCredsOptions) run(cmd *cobra.Command, args []string) error {
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

	oldState, err := c.LoadZarfState(ctx)
	if err != nil {
		return err
	}
	// TODO: Determine if this is actually needed.
	if oldState.Distro == "" {
		return errors.New("zarf state secret did not load properly")
	}
	newState, err := cluster.MergeZarfState(oldState, updateCredsInitOpts, args)
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

	// Update Zarf 'init' component Helm releases if present
	h := helm.NewClusterOnly(&types.PackagerConfig{}, template.GetZarfVariableConfig(cmd.Context()), newState, c)

	if slices.Contains(args, message.RegistryKey) && newState.RegistryInfo.IsInternal() {
		err = h.UpdateZarfRegistryValues(ctx)
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
		err = h.UpdateZarfAgentValues(ctx)
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
			l.Info("registry push password", "changed", !(oR.PushPassword == nR.PushPassword))
			l.Info("registry pull username", "existing", oR.PullUsername, "replacement", nR.PullUsername)
			l.Info("registry pull password", "changed", !(oR.PullPassword == nR.PullPassword))
		case gitKey:
			oG := oldState.GitServer
			nG := newState.GitServer
			l.Info("Git server URL address", "existing", oG.Address, "replacement", nG.Address)
			l.Info("Git server push username", "existing", oG.PushUsername, "replacement", nG.PushUsername)
			l.Info("Git server push password", "changed", !(oG.PushPassword == nG.PushPassword))
			l.Info("Git server pull username", "existing", oG.PullUsername, "replacement", nG.PullUsername)
			l.Info("Git server pull password", "changed", !(oG.PullPassword == nG.PullPassword))
		case artifactKey:
			oA := oldState.ArtifactServer
			nA := newState.ArtifactServer
			l.Info("artifact server URL address", "existing", oA.Address, "replacement", nA.Address)
			l.Info("artifact server push username", "existing", oA.PushUsername, "replacement", nA.PushUsername)
			l.Info("artifact server push token", "changed", !(oA.PushToken == nA.PushToken))
		case agentKey:
			oT := oldState.AgentTLS
			nT := newState.AgentTLS
			l.Info("agent certificate authority", "changed", !(string(oT.CA) == string(nT.CA)))
			l.Info("agent public certificate", "changed", !(string(oT.Cert) == string(nT.Cert)))
			l.Info("agent private key", "changed", !(string(oT.Key) == string(nT.Key)))
		}
	}
}

// ClearCacheOptions holds the command-line options for 'tools clear-cache' sub-command.
type ClearCacheOptions struct{}

// NewClearCacheCommand creates the `tools clear-cache` sub-command.
func NewClearCacheCommand() *cobra.Command {
	o := &ClearCacheOptions{}

	cmd := &cobra.Command{
		Use:     "clear-cache",
		Aliases: []string{"c"},
		Short:   lang.CmdToolsClearCacheShort,
		RunE:    o.run,
	}

	cmd.Flags().StringVar(&config.CommonOptions.CachePath, "zarf-cache", config.ZarfDefaultCachePath, lang.CmdToolsClearCacheFlagCachePath)

	return cmd
}

// Run performs the execution of 'tools clear-cache' sub-command.
func (o *ClearCacheOptions) run(cmd *cobra.Command, _ []string) error {
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

// DownloadInitOptions holds the command-line options for 'tools download-init' sub-command.
type DownloadInitOptions struct{}

// NewDownloadInitCommand creates the `tools download-init` sub-command.
func NewDownloadInitCommand() *cobra.Command {
	o := &DownloadInitOptions{}

	cmd := &cobra.Command{
		Use:   "download-init",
		Short: lang.CmdToolsDownloadInitShort,
		RunE:  o.run,
	}

	cmd.Flags().StringVarP(&outputDirectory, "output-directory", "o", "", lang.CmdToolsDownloadInitFlagOutputDirectory)

	return cmd
}

// Run performs the execution of 'tools download-init' sub-command.
func (o *DownloadInitOptions) run(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	url := zoci.GetInitPackageURL(config.CLIVersion)
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

// GenPKIOptions holds the command-line options for 'tools gen-pki' sub-command.
type GenPKIOptions struct{}

// NewGenPKICommand creates the `tools gen-pki` sub-command.
func NewGenPKICommand() *cobra.Command {
	o := &GenPKIOptions{}

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

// Run performs the execution of 'tools gen-pki' sub-command.
func (o *GenPKIOptions) run(cmd *cobra.Command, args []string) error {
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

// GenKeyOptions holds the command-line options for 'tools gen-key' sub-command.
type GenKeyOptions struct{}

// NewGenKeyCommand creates the `tools gen-key` sub-command.
func NewGenKeyCommand() *cobra.Command {
	o := &GenKeyOptions{}

	cmd := &cobra.Command{
		Use:     "gen-key",
		Aliases: []string{"key"},
		Short:   lang.CmdToolsGenKeyShort,
		RunE:    o.run,
	}

	return cmd
}

// Run performs the execution of 'tools gen-key' sub-command.
func (o *GenKeyOptions) run(cmd *cobra.Command, _ []string) error {
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
