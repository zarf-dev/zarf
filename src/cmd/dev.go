// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/mholt/archiver/v3"
	"github.com/pterm/pterm"
	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/zarf-dev/zarf/src/cmd/common"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/packager"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/types"
)

var defaultRegistry = fmt.Sprintf("%s:%d", helpers.IPV4Localhost, types.ZarfInClusterContainerRegistryNodePort)

func newDevCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "dev",
		Aliases: []string{"prepare", "prep"},
		Short:   lang.CmdDevShort,
	}

	v := common.GetViper()

	cmd.AddCommand(newDevDeployCommand(v))
	cmd.AddCommand(newDevGenerateCommand())
	cmd.AddCommand(newDevPatchGitCommand())
	cmd.AddCommand(newDevSha256SumCommand())
	cmd.AddCommand(newDevFindImagesCommand(v))
	cmd.AddCommand(newDevGenerateConfigCommand())
	cmd.AddCommand(newDevLintCommand(v))

	return cmd
}

type devDeployOptions struct{}

func newDevDeployCommand(v *viper.Viper) *cobra.Command {
	o := &devDeployOptions{}

	cmd := &cobra.Command{
		Use:   "deploy",
		Args:  cobra.MaximumNArgs(1),
		Short: lang.CmdDevDeployShort,
		Long:  lang.CmdDevDeployLong,
		RunE:  o.run,
	}

	// TODO(soltysh): get rid of pkgConfig global
	cmd.Flags().StringToStringVar(&pkgConfig.CreateOpts.SetVariables, "create-set", v.GetStringMapString(common.VPkgCreateSet), lang.CmdPackageCreateFlagSet)
	cmd.Flags().StringToStringVar(&pkgConfig.CreateOpts.RegistryOverrides, "registry-override", v.GetStringMapString(common.VPkgCreateRegistryOverride), lang.CmdPackageCreateFlagRegistryOverride)
	cmd.Flags().StringVarP(&pkgConfig.CreateOpts.Flavor, "flavor", "f", v.GetString(common.VPkgCreateFlavor), lang.CmdPackageCreateFlagFlavor)

	cmd.Flags().StringVar(&pkgConfig.DeployOpts.RegistryURL, "registry-url", defaultRegistry, lang.CmdDevFlagRegistry)
	err := cmd.Flags().MarkHidden("registry-url")
	if err != nil {
		logger.Default().Debug("unable to mark dev-deploy flag as hidden", "error", err)
	}

	cmd.Flags().StringToStringVar(&pkgConfig.PkgOpts.SetVariables, "deploy-set", v.GetStringMapString(common.VPkgDeploySet), lang.CmdPackageDeployFlagSet)

	// Always require adopt-existing-resources flag (no viper)
	cmd.Flags().BoolVar(&pkgConfig.DeployOpts.AdoptExistingResources, "adopt-existing-resources", false, lang.CmdPackageDeployFlagAdoptExistingResources)
	cmd.Flags().DurationVar(&pkgConfig.DeployOpts.Timeout, "timeout", v.GetDuration(common.VPkgDeployTimeout), lang.CmdPackageDeployFlagTimeout)

	cmd.Flags().IntVar(&pkgConfig.PkgOpts.Retries, "retries", v.GetInt(common.VPkgRetries), lang.CmdPackageFlagRetries)
	cmd.Flags().StringVar(&pkgConfig.PkgOpts.OptionalComponents, "components", v.GetString(common.VPkgDeployComponents), lang.CmdPackageDeployFlagComponents)

	cmd.Flags().BoolVar(&pkgConfig.CreateOpts.NoYOLO, "no-yolo", v.GetBool(common.VDevDeployNoYolo), lang.CmdDevDeployFlagNoYolo)

	return cmd
}

func (o *devDeployOptions) run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	pkgConfig.CreateOpts.BaseDir = setBaseDirectory(args)

	v := common.GetViper()
	pkgConfig.CreateOpts.SetVariables = helpers.TransformAndMergeMap(
		v.GetStringMapString(common.VPkgCreateSet), pkgConfig.CreateOpts.SetVariables, strings.ToUpper)

	pkgConfig.PkgOpts.SetVariables = helpers.TransformAndMergeMap(
		v.GetStringMapString(common.VPkgDeploySet), pkgConfig.PkgOpts.SetVariables, strings.ToUpper)

	pkgClient, err := packager.New(&pkgConfig, packager.WithContext(ctx))
	if err != nil {
		return err
	}
	defer pkgClient.ClearTempPaths()

	err = pkgClient.DevDeploy(ctx)
	var lintErr *lint.LintError
	if errors.As(err, &lintErr) {
		common.PrintFindings(ctx, lintErr)
	}
	if err != nil {
		return fmt.Errorf("failed to dev deploy: %w", err)
	}

	return nil
}

type devGenerateOptions struct{}

func newDevGenerateCommand() *cobra.Command {
	o := &devGenerateOptions{}

	cmd := &cobra.Command{
		Use:     "generate NAME",
		Aliases: []string{"g"},
		Args:    cobra.ExactArgs(1),
		Short:   lang.CmdDevGenerateShort,
		Example: lang.CmdDevGenerateExample,
		RunE:    o.run,
	}

	cmd.Flags().StringVar(&pkgConfig.GenerateOpts.URL, "url", "", "URL to the source git repository")
	cmd.MarkFlagRequired("url")
	cmd.Flags().StringVar(&pkgConfig.GenerateOpts.Version, "version", "", "The Version of the chart to use")
	cmd.MarkFlagRequired("version")
	cmd.Flags().StringVar(&pkgConfig.GenerateOpts.GitPath, "gitPath", "", "Relative path to the chart in the git repository")
	cmd.Flags().StringVar(&pkgConfig.GenerateOpts.Output, "output-directory", "", "Output directory for the generated zarf.yaml")
	cmd.MarkFlagRequired("output-directory")
	cmd.Flags().StringVar(&pkgConfig.FindImagesOpts.KubeVersionOverride, "kube-version", "", lang.CmdDevFlagKubeVersion)

	return cmd
}

func (o *devGenerateOptions) run(cmd *cobra.Command, args []string) error {
	pkgConfig.GenerateOpts.Name = args[0]

	pkgConfig.CreateOpts.BaseDir = "."
	pkgConfig.FindImagesOpts.RepoHelmChartPath = pkgConfig.GenerateOpts.GitPath

	pkgClient, err := packager.New(&pkgConfig, packager.WithContext(cmd.Context()))
	if err != nil {
		return err
	}
	defer pkgClient.ClearTempPaths()

	err = pkgClient.Generate(cmd.Context())
	if err != nil {
		return err
	}
	return nil
}

type devPatchGitOptions struct{}

func newDevPatchGitCommand() *cobra.Command {
	o := &devPatchGitOptions{}

	cmd := &cobra.Command{
		Use:     "patch-git HOST FILE",
		Aliases: []string{"p"},
		Short:   lang.CmdDevPatchGitShort,
		Args:    cobra.ExactArgs(2),
		RunE:    o.run,
	}

	// TODO(soltysh): get rid of pkgConfig global
	cmd.Flags().StringVar(&pkgConfig.InitOpts.GitServer.PushUsername, "git-account", types.ZarfGitPushUser, lang.CmdDevFlagGitAccount)

	return cmd
}

func (o *devPatchGitOptions) run(_ *cobra.Command, args []string) error {
	host, fileName := args[0], args[1]

	// Read the contents of the given file
	content, err := os.ReadFile(fileName)
	if err != nil {
		return fmt.Errorf("unable to read the file %s: %w", fileName, err)
	}

	gitServer := pkgConfig.InitOpts.GitServer
	gitServer.Address = host

	// Perform git url transformation via regex
	text := string(content)

	// TODO(mkcp): Currently uses message for its log fn. Migrate to ctx and slog
	processedText := transform.MutateGitURLsInText(message.Warnf, gitServer.Address, text, gitServer.PushUsername)

	// Print the differences
	// TODO(mkcp): Uses pterm to print text diffs. Decouple from pterm after we release logger.
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(text, processedText, true)
	diffs = dmp.DiffCleanupSemantic(diffs)
	pterm.Println(dmp.DiffPrettyText(diffs))

	// Ask the user before this destructive action
	confirm := false
	prompt := &survey.Confirm{
		Message: fmt.Sprintf(lang.CmdDevPatchGitOverwritePrompt, fileName),
	}
	if err := survey.AskOne(prompt, &confirm); err != nil {
		return fmt.Errorf("confirm overwrite canceled: %w", err)
	}

	if confirm {
		// Overwrite the file
		err = os.WriteFile(fileName, []byte(processedText), helpers.ReadAllWriteUser)
		if err != nil {
			return fmt.Errorf("unable to write the changes back to the file: %w", err)
		}
	}

	return nil
}

type devSha256SumOptions struct {
	extractPath string
}

func newDevSha256SumCommand() *cobra.Command {
	o := &devSha256SumOptions{}

	cmd := &cobra.Command{
		Use:     "sha256sum { FILE | URL }",
		Aliases: []string{"s"},
		Short:   lang.CmdDevSha256sumShort,
		Args:    cobra.ExactArgs(1),
		RunE:    o.run,
	}

	cmd.Flags().StringVarP(&o.extractPath, "extract-path", "e", "", lang.CmdDevFlagExtractPath)

	return cmd
}

func (o *devSha256SumOptions) run(cmd *cobra.Command, args []string) (err error) {
	hashErr := errors.New("unable to compute the SHA256SUM hash")

	fileName := args[0]

	var tmp string
	var data io.ReadCloser

	if helpers.IsURL(fileName) {
		message.Warn(lang.CmdDevSha256sumRemoteWarning)
		logger.From(cmd.Context()).Warn("this is a remote source. If a published checksum is available you should use that rather than calculating it directly from the remote link")

		fileBase, err := helpers.ExtractBasePathFromURL(fileName)
		if err != nil {
			return errors.Join(hashErr, err)
		}

		if fileBase == "" {
			fileBase = "sha-file"
		}

		tmp, err = utils.MakeTempDir(config.CommonOptions.TempDirectory)
		if err != nil {
			return errors.Join(hashErr, err)
		}

		downloadPath := filepath.Join(tmp, fileBase)
		err = utils.DownloadToFile(cmd.Context(), fileName, downloadPath, "")
		if err != nil {
			return errors.Join(hashErr, err)
		}

		fileName = downloadPath

		defer func(path string) {
			errRemove := os.RemoveAll(path)
			err = errors.Join(err, errRemove)
		}(tmp)
	}

	if o.extractPath != "" {
		if tmp == "" {
			tmp, err = utils.MakeTempDir(config.CommonOptions.TempDirectory)
			if err != nil {
				return errors.Join(hashErr, err)
			}
			defer func(path string) {
				errRemove := os.RemoveAll(path)
				err = errors.Join(err, errRemove)
			}(tmp)
		}

		extractedFile := filepath.Join(tmp, o.extractPath)

		err = archiver.Extract(fileName, o.extractPath, tmp)
		if err != nil {
			return errors.Join(hashErr, err)
		}

		fileName = extractedFile
	}

	data, err = os.Open(fileName)
	if err != nil {
		return errors.Join(hashErr, err)
	}
	defer func(data io.ReadCloser) {
		errClose := data.Close()
		err = errors.Join(err, errClose)
	}(data)

	hash, err := helpers.GetSHA256Hash(data)
	if err != nil {
		return errors.Join(hashErr, err)
	}
	fmt.Println(hash)
	return nil
}

type devFindImagesOptions struct{}

func newDevFindImagesCommand(v *viper.Viper) *cobra.Command {
	o := &devFindImagesOptions{}

	cmd := &cobra.Command{
		Use:     "find-images [ DIRECTORY ]",
		Aliases: []string{"f"},
		Args:    cobra.MaximumNArgs(1),
		Short:   lang.CmdDevFindImagesShort,
		Long:    lang.CmdDevFindImagesLong,
		RunE:    o.run,
	}

	// TODO(soltysh): get rid of pkgConfig global
	cmd.Flags().StringVarP(&pkgConfig.FindImagesOpts.RepoHelmChartPath, "repo-chart-path", "p", "", lang.CmdDevFlagRepoChartPath)
	// use the package create config for this and reset it here to avoid overwriting the config.CreateOptions.SetVariables
	cmd.Flags().StringToStringVar(&pkgConfig.CreateOpts.SetVariables, "set", v.GetStringMapString(common.VPkgCreateSet), lang.CmdDevFlagSet)

	err := cmd.Flags().MarkDeprecated("set", "this field is replaced by create-set")
	if err != nil {
		logger.Default().Debug("unable to mark dev-find-images flag as set", "error", err)
	}
	err = cmd.Flags().MarkHidden("set")
	if err != nil {
		logger.Default().Debug("unable to mark dev-find-images flag as hidden", "error", err)
	}
	cmd.Flags().StringVarP(&pkgConfig.CreateOpts.Flavor, "flavor", "f", v.GetString(common.VPkgCreateFlavor), lang.CmdPackageCreateFlagFlavor)
	cmd.Flags().StringToStringVar(&pkgConfig.CreateOpts.SetVariables, "create-set", v.GetStringMapString(common.VPkgCreateSet), lang.CmdDevFlagSet)
	cmd.Flags().StringToStringVar(&pkgConfig.PkgOpts.SetVariables, "deploy-set", v.GetStringMapString(common.VPkgDeploySet), lang.CmdPackageDeployFlagSet)
	// allow for the override of the default helm KubeVersion
	cmd.Flags().StringVar(&pkgConfig.FindImagesOpts.KubeVersionOverride, "kube-version", "", lang.CmdDevFlagKubeVersion)
	// check which manifests are using this particular image
	cmd.Flags().StringVar(&pkgConfig.FindImagesOpts.Why, "why", "", lang.CmdDevFlagFindImagesWhy)
	// skip searching cosign artifacts in find images
	cmd.Flags().BoolVar(&pkgConfig.FindImagesOpts.SkipCosign, "skip-cosign", false, lang.CmdDevFlagFindImagesSkipCosign)

	cmd.Flags().StringVar(&pkgConfig.FindImagesOpts.RegistryURL, "registry-url", defaultRegistry, lang.CmdDevFlagRegistry)

	return cmd
}

func (o *devFindImagesOptions) run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	pkgConfig.CreateOpts.BaseDir = setBaseDirectory(args)

	v := common.GetViper()

	pkgConfig.CreateOpts.SetVariables = helpers.TransformAndMergeMap(
		v.GetStringMapString(common.VPkgCreateSet), pkgConfig.CreateOpts.SetVariables, strings.ToUpper)
	pkgConfig.PkgOpts.SetVariables = helpers.TransformAndMergeMap(
		v.GetStringMapString(common.VPkgDeploySet), pkgConfig.PkgOpts.SetVariables, strings.ToUpper)
	pkgClient, err := packager.New(&pkgConfig, packager.WithContext(cmd.Context()))
	if err != nil {
		return err
	}
	defer pkgClient.ClearTempPaths()

	_, err = pkgClient.FindImages(ctx)

	var lintErr *lint.LintError
	if errors.As(err, &lintErr) {
		common.PrintFindings(ctx, lintErr)
	}
	if err != nil {
		return fmt.Errorf("unable to find images: %w", err)
	}
	return nil
}

type devGenerateConfigOptions struct{}

func newDevGenerateConfigCommand() *cobra.Command {
	o := &devGenerateConfigOptions{}

	cmd := &cobra.Command{
		Use:     "generate-config [ FILENAME ]",
		Aliases: []string{"gc"},
		Args:    cobra.MaximumNArgs(1),
		Short:   lang.CmdDevGenerateConfigShort,
		Long:    lang.CmdDevGenerateConfigLong,
		RunE:    o.run,
	}

	return cmd
}

func (o *devGenerateConfigOptions) run(_ *cobra.Command, args []string) error {
	// If a filename was provided, use that
	fileName := "zarf-config.toml"
	if len(args) > 0 {
		fileName = args[0]
	}

	v := common.GetViper()
	if err := v.SafeWriteConfigAs(fileName); err != nil {
		return fmt.Errorf("unable to write the config file %s, make sure the file doesn't already exist: %w", fileName, err)
	}
	return nil
}

type devLintOptions struct{}

func newDevLintCommand(v *viper.Viper) *cobra.Command {
	o := &devLintOptions{}

	cmd := &cobra.Command{
		Use:     "lint [ DIRECTORY ]",
		Args:    cobra.MaximumNArgs(1),
		Aliases: []string{"l"},
		Short:   lang.CmdDevLintShort,
		Long:    lang.CmdDevLintLong,
		RunE:    o.run,
	}

	cmd.Flags().StringToStringVar(&pkgConfig.CreateOpts.SetVariables, "set", v.GetStringMapString(common.VPkgCreateSet), lang.CmdPackageCreateFlagSet)
	cmd.Flags().StringVarP(&pkgConfig.CreateOpts.Flavor, "flavor", "f", v.GetString(common.VPkgCreateFlavor), lang.CmdPackageCreateFlagFlavor)

	return cmd
}

func (o *devLintOptions) run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	config.CommonOptions.Confirm = true
	pkgConfig.CreateOpts.BaseDir = setBaseDirectory(args)
	v := common.GetViper()
	pkgConfig.CreateOpts.SetVariables = helpers.TransformAndMergeMap(
		v.GetStringMapString(common.VPkgCreateSet), pkgConfig.CreateOpts.SetVariables, strings.ToUpper)

	err := lint.Validate(ctx, pkgConfig.CreateOpts.BaseDir, pkgConfig.CreateOpts.Flavor, pkgConfig.CreateOpts.SetVariables)
	var lintErr *lint.LintError
	if errors.As(err, &lintErr) {
		common.PrintFindings(ctx, lintErr)
		// Do not return an error if the findings are all warnings.
		if lintErr.OnlyWarnings() {
			return nil
		}
	}
	if err != nil {
		return err
	}
	return nil
}
