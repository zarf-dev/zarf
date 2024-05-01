// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/fatih/color"
	goyaml "github.com/goccy/go-yaml"
	"github.com/pterm/pterm"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/pkg/helpers"
	"github.com/defenseunicorns/zarf/src/cmd/common"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/packager"
	"github.com/defenseunicorns/zarf/src/pkg/packager/lint"
	"github.com/defenseunicorns/zarf/src/pkg/packager/migrations"
	"github.com/defenseunicorns/zarf/src/pkg/transform"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/mholt/archiver/v3"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var extractPath string
var deprecatedMigrationsToRun []string
var featureMigrationsToRun []string

var devCmd = &cobra.Command{
	Use:     "dev",
	Aliases: []string{"prepare", "prep"},
	Short:   lang.CmdDevShort,
}

var devDeployCmd = &cobra.Command{
	Use:   "deploy",
	Args:  cobra.MaximumNArgs(1),
	Short: lang.CmdDevDeployShort,
	Long:  lang.CmdDevDeployLong,
	Run: func(_ *cobra.Command, args []string) {
		pkgConfig.CreateOpts.BaseDir = common.SetBaseDirectory(args)

		v := common.GetViper()
		pkgConfig.CreateOpts.SetVariables = helpers.TransformAndMergeMap(
			v.GetStringMapString(common.VPkgCreateSet), pkgConfig.CreateOpts.SetVariables, strings.ToUpper)

		pkgConfig.PkgOpts.SetVariables = helpers.TransformAndMergeMap(
			v.GetStringMapString(common.VPkgDeploySet), pkgConfig.PkgOpts.SetVariables, strings.ToUpper)

		// Configure the packager
		pkgClient := packager.NewOrDie(&pkgConfig)
		defer pkgClient.ClearTempPaths()

		// Create the package
		if err := pkgClient.DevDeploy(); err != nil {
			message.Fatalf(err, lang.CmdDevDeployErr, err.Error())
		}
	},
}

var devMigrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: lang.CmdDevMigrateShort,
	Args:  cobra.MaximumNArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		dir := common.SetBaseDirectory(args)
		var pkg types.ZarfPackage
		cm := goyaml.CommentMap{}

		before, err := os.ReadFile(filepath.Join(dir, layout.ZarfYAML))
		if err != nil {
			message.Fatalf(err, lang.CmdDevMigrateErr, err.Error())
		}

		if err := goyaml.UnmarshalWithOptions(before, &pkg, goyaml.CommentToMap(cm)); err != nil {
			message.Fatalf(err, lang.CmdDevMigrateErr, err.Error())
		}

		data := [][]string{}

		deprecatedMigrationsToRun = slices.Compact(deprecatedMigrationsToRun)
		for _, m := range migrations.DeprecatedComponentMigrations() {
			if !slices.Contains(deprecatedMigrationsToRun, m.String()) && len(deprecatedMigrationsToRun) != 0 {
				continue
			}

			entry := []string{
				m.String(),
				message.ColorWrap("deprecated", color.FgYellow),
				"",
			}

			for idx, component := range pkg.Components {
				mc, _ := m.Run(component)
				mc = m.Clear(mc)
				if !reflect.DeepEqual(mc, component) {
					entry[2] = fmt.Sprintf(".components.[%d]", idx)
					data = append(data, entry)
				}
				pkg.Components[idx] = mc
			}
		}

		featureMigrationsToRun = slices.Compact(featureMigrationsToRun)
		for _, m := range migrations.FeatureMigrations() {
			if !slices.Contains(featureMigrationsToRun, m.String()) {
				continue
			}
			pkgWithFeature, warning := m.Run(pkg)
			if warning != "" {
				message.Warn(warning)
			}
			if !reflect.DeepEqual(pkgWithFeature, pkg) {
				data = append(data, []string{
					m.String(),
					message.ColorWrap("feature", color.FgMagenta),
					".",
				})
			}
			pkg = pkgWithFeature
		}

		after, err := goyaml.MarshalWithOptions(pkg, goyaml.WithComment(cm), goyaml.IndentSequence(true), goyaml.UseSingleQuote(false))
		if err != nil {
			message.Fatalf(err, lang.CmdDevMigrateErr, err.Error())
		}

		header := []string{
			"Migration",
			"Type",
			"Affected Path(s)",
		}

		if len(data) == 0 {
			message.Warn("No changes made")
		} else {
			pterm.Println()
			fmt.Println(string(after))
			message.Table(header, data)
		}
	},
}

var devGenerateCmd = &cobra.Command{
	Use:     "generate NAME",
	Aliases: []string{"g"},
	Args:    cobra.ExactArgs(1),
	Short:   lang.CmdDevGenerateShort,
	Example: lang.CmdDevGenerateExample,
	Run: func(_ *cobra.Command, args []string) {
		pkgConfig.GenerateOpts.Name = args[0]

		pkgConfig.CreateOpts.BaseDir = "."
		pkgConfig.FindImagesOpts.RepoHelmChartPath = pkgConfig.GenerateOpts.GitPath

		pkgClient := packager.NewOrDie(&pkgConfig)
		defer pkgClient.ClearTempPaths()

		if err := pkgClient.Generate(); err != nil {
			message.Fatalf(err, err.Error())
		}
	},
}

var devTransformGitLinksCmd = &cobra.Command{
	Use:     "patch-git HOST FILE",
	Aliases: []string{"p"},
	Short:   lang.CmdDevPatchGitShort,
	Args:    cobra.ExactArgs(2),
	Run: func(_ *cobra.Command, args []string) {
		host, fileName := args[0], args[1]

		// Read the contents of the given file
		content, err := os.ReadFile(fileName)
		if err != nil {
			message.Fatalf(err, lang.CmdDevPatchGitFileReadErr, fileName)
		}

		pkgConfig.InitOpts.GitServer.Address = host

		// Perform git url transformation via regex
		text := string(content)
		processedText := transform.MutateGitURLsInText(message.Warnf, pkgConfig.InitOpts.GitServer.Address, text, pkgConfig.InitOpts.GitServer.PushUsername)

		// Print the differences
		message.PrintDiff(text, processedText)

		// Ask the user before this destructive action
		confirm := false
		prompt := &survey.Confirm{
			Message: fmt.Sprintf(lang.CmdDevPatchGitOverwritePrompt, fileName),
		}
		if err := survey.AskOne(prompt, &confirm); err != nil {
			message.Fatalf(nil, lang.CmdDevPatchGitOverwriteErr, err.Error())
		}

		if confirm {
			// Overwrite the file
			err = os.WriteFile(fileName, []byte(processedText), helpers.ReadAllWriteUser)
			if err != nil {
				message.Fatal(err, lang.CmdDevPatchGitFileWriteErr)
			}
		}

	},
}

var devSha256SumCmd = &cobra.Command{
	Use:     "sha256sum { FILE | URL }",
	Aliases: []string{"s"},
	Short:   lang.CmdDevSha256sumShort,
	Args:    cobra.ExactArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		fileName := args[0]

		var tmp string
		var data io.ReadCloser
		var err error

		if helpers.IsURL(fileName) {
			message.Warn(lang.CmdDevSha256sumRemoteWarning)

			fileBase, err := helpers.ExtractBasePathFromURL(fileName)
			if err != nil {
				message.Fatalf(err, lang.CmdDevSha256sumHashErr, err.Error())
			}

			if fileBase == "" {
				fileBase = "sha-file"
			}

			tmp, err = utils.MakeTempDir(config.CommonOptions.TempDirectory)
			if err != nil {
				message.Fatalf(err, lang.CmdDevSha256sumHashErr, err.Error())
			}

			downloadPath := filepath.Join(tmp, fileBase)
			err = utils.DownloadToFile(fileName, downloadPath, "")
			if err != nil {
				message.Fatalf(err, lang.CmdDevSha256sumHashErr, err.Error())
			}

			fileName = downloadPath

			defer os.RemoveAll(tmp)
		}

		if extractPath != "" {
			if tmp == "" {
				tmp, err = utils.MakeTempDir(config.CommonOptions.TempDirectory)
				if err != nil {
					message.Fatalf(err, lang.CmdDevSha256sumHashErr, err.Error())
				}
				defer os.RemoveAll(tmp)
			}

			extractedFile := filepath.Join(tmp, extractPath)

			err = archiver.Extract(fileName, extractPath, tmp)
			if err != nil {
				message.Fatalf(err, lang.CmdDevSha256sumHashErr, err.Error())
			}

			fileName = extractedFile
		}

		data, err = os.Open(fileName)
		if err != nil {
			message.Fatalf(err, lang.CmdDevSha256sumHashErr, err.Error())
		}
		defer data.Close()

		var hash string
		hash, err = helpers.GetSHA256Hash(data)
		if err != nil {
			message.Fatalf(err, lang.CmdDevSha256sumHashErr, err.Error())
		} else {
			fmt.Println(hash)
		}
	},
}

var devFindImagesCmd = &cobra.Command{
	Use:     "find-images [ PACKAGE ]",
	Aliases: []string{"f"},
	Args:    cobra.MaximumNArgs(1),
	Short:   lang.CmdDevFindImagesShort,
	Long:    lang.CmdDevFindImagesLong,
	Run: func(_ *cobra.Command, args []string) {
		pkgConfig.CreateOpts.BaseDir = common.SetBaseDirectory(args)

		// Ensure uppercase keys from viper
		v := common.GetViper()

		pkgConfig.CreateOpts.SetVariables = helpers.TransformAndMergeMap(
			v.GetStringMapString(common.VPkgCreateSet), pkgConfig.CreateOpts.SetVariables, strings.ToUpper)
		pkgConfig.PkgOpts.SetVariables = helpers.TransformAndMergeMap(
			v.GetStringMapString(common.VPkgDeploySet), pkgConfig.PkgOpts.SetVariables, strings.ToUpper)

		// Configure the packager
		pkgClient := packager.NewOrDie(&pkgConfig)
		defer pkgClient.ClearTempPaths()

		// Find all the images the package might need
		if _, err := pkgClient.FindImages(); err != nil {
			message.Fatalf(err, lang.CmdDevFindImagesErr, err.Error())
		}
	},
}

var devGenConfigFileCmd = &cobra.Command{
	Use:     "generate-config [ FILENAME ]",
	Aliases: []string{"gc"},
	Args:    cobra.MaximumNArgs(1),
	Short:   lang.CmdDevGenerateConfigShort,
	Long:    lang.CmdDevGenerateConfigLong,
	Run: func(_ *cobra.Command, args []string) {
		fileName := "zarf-config.toml"

		// If a filename was provided, use that
		if len(args) > 0 {
			fileName = args[0]
		}

		v := common.GetViper()
		if err := v.SafeWriteConfigAs(fileName); err != nil {
			message.Fatalf(err, lang.CmdDevGenerateConfigErr, fileName)
		}
	},
}

var devLintCmd = &cobra.Command{
	Use:     "lint [ DIRECTORY ]",
	Args:    cobra.MaximumNArgs(1),
	Aliases: []string{"l"},
	Short:   lang.CmdDevLintShort,
	Long:    lang.CmdDevLintLong,
	Run: func(_ *cobra.Command, args []string) {
		pkgConfig.CreateOpts.BaseDir = common.SetBaseDirectory(args)
		v := common.GetViper()
		pkgConfig.CreateOpts.SetVariables = helpers.TransformAndMergeMap(
			v.GetStringMapString(common.VPkgCreateSet), pkgConfig.CreateOpts.SetVariables, strings.ToUpper)
		validator, err := lint.Validate(pkgConfig.CreateOpts)
		if err != nil {
			message.Fatal(err, err.Error())
		}
		validator.DisplayFormattedMessage()
		if !validator.IsSuccess() {
			os.Exit(1)
		}
	},
}

func init() {
	v := common.GetViper()
	rootCmd.AddCommand(devCmd)

	devCmd.AddCommand(devDeployCmd)
	devCmd.AddCommand(devMigrateCmd)
	devCmd.AddCommand(devGenerateCmd)
	devCmd.AddCommand(devTransformGitLinksCmd)
	devCmd.AddCommand(devSha256SumCmd)
	devCmd.AddCommand(devFindImagesCmd)
	devCmd.AddCommand(devGenConfigFileCmd)
	devCmd.AddCommand(devLintCmd)

	bindDevDeployFlags(v)
	bindDevGenerateFlags(v)

	cm := []string{}
	for _, m := range migrations.DeprecatedComponentMigrations() {
		cm = append(cm, m.String())
	}
	devMigrateCmd.Flags().StringArrayVar(&deprecatedMigrationsToRun, "run", []string{}, fmt.Sprintf("migrations of deprecated features to run (default: all, available: %s)", strings.Join(cm, ", ")))
	devMigrateCmd.RegisterFlagCompletionFunc("run", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		ids := []string{}
		for _, m := range migrations.DeprecatedComponentMigrations() {
			if slices.Contains(deprecatedMigrationsToRun, m.String()) {
				continue
			}
			ids = append(ids, m.String())
		}
		return ids, cobra.ShellCompDirectiveNoFileComp
	})

	fm := []string{}
	for _, m := range migrations.FeatureMigrations() {
		fm = append(fm, m.String())
	}
	devMigrateCmd.Flags().StringArrayVar(&featureMigrationsToRun, "enable-feature", []string{}, fmt.Sprintf("feature migrations to run and enable (available: %s)", strings.Join(fm, ", ")))
	devMigrateCmd.RegisterFlagCompletionFunc("enable-feature", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		ids := []string{}
		for _, m := range migrations.FeatureMigrations() {
			if slices.Contains(featureMigrationsToRun, m.String()) {
				continue
			}
			ids = append(ids, m.String())
		}
		return ids, cobra.ShellCompDirectiveNoFileComp
	})

	devSha256SumCmd.Flags().StringVarP(&extractPath, "extract-path", "e", "", lang.CmdDevFlagExtractPath)

	devFindImagesCmd.Flags().StringVarP(&pkgConfig.FindImagesOpts.RepoHelmChartPath, "repo-chart-path", "p", "", lang.CmdDevFlagRepoChartPath)
	// use the package create config for this and reset it here to avoid overwriting the config.CreateOptions.SetVariables
	devFindImagesCmd.Flags().StringToStringVar(&pkgConfig.CreateOpts.SetVariables, "set", v.GetStringMapString(common.VPkgCreateSet), lang.CmdDevFlagSet)

	devFindImagesCmd.Flags().MarkDeprecated("set", "this field is replaced by create-set")
	devFindImagesCmd.Flags().MarkHidden("set")
	devFindImagesCmd.Flags().StringVarP(&pkgConfig.CreateOpts.Flavor, "flavor", "f", v.GetString(common.VPkgCreateFlavor), lang.CmdPackageCreateFlagFlavor)
	devFindImagesCmd.Flags().StringToStringVar(&pkgConfig.CreateOpts.SetVariables, "create-set", v.GetStringMapString(common.VPkgCreateSet), lang.CmdDevFlagSet)
	devFindImagesCmd.Flags().StringToStringVar(&pkgConfig.PkgOpts.SetVariables, "deploy-set", v.GetStringMapString(common.VPkgDeploySet), lang.CmdPackageDeployFlagSet)
	// allow for the override of the default helm KubeVersion
	devFindImagesCmd.Flags().StringVar(&pkgConfig.FindImagesOpts.KubeVersionOverride, "kube-version", "", lang.CmdDevFlagKubeVersion)
	// check which manifests are using this particular image
	devFindImagesCmd.Flags().StringVar(&pkgConfig.FindImagesOpts.Why, "why", "", lang.CmdDevFlagFindImagesWhy)
	// skip searching cosign artifacts in find images
	devFindImagesCmd.Flags().BoolVar(&pkgConfig.FindImagesOpts.SkipCosign, "skip-cosign", false, lang.CmdDevFlagFindImagesSkipCosign)

	defaultRegistry := fmt.Sprintf("%s:%d", helpers.IPV4Localhost, types.ZarfInClusterContainerRegistryNodePort)
	devFindImagesCmd.Flags().StringVar(&pkgConfig.FindImagesOpts.RegistryURL, "registry-url", defaultRegistry, lang.CmdDevFlagFindImagesRegistry)

	devLintCmd.Flags().StringToStringVar(&pkgConfig.CreateOpts.SetVariables, "set", v.GetStringMapString(common.VPkgCreateSet), lang.CmdPackageCreateFlagSet)
	devLintCmd.Flags().StringVarP(&pkgConfig.CreateOpts.Flavor, "flavor", "f", v.GetString(common.VPkgCreateFlavor), lang.CmdPackageCreateFlagFlavor)
	devTransformGitLinksCmd.Flags().StringVar(&pkgConfig.InitOpts.GitServer.PushUsername, "git-account", types.ZarfGitPushUser, lang.CmdDevFlagGitAccount)
}

func bindDevDeployFlags(v *viper.Viper) {
	devDeployFlags := devDeployCmd.Flags()

	devDeployFlags.StringToStringVar(&pkgConfig.CreateOpts.SetVariables, "create-set", v.GetStringMapString(common.VPkgCreateSet), lang.CmdPackageCreateFlagSet)
	devDeployFlags.StringToStringVar(&pkgConfig.CreateOpts.RegistryOverrides, "registry-override", v.GetStringMapString(common.VPkgCreateRegistryOverride), lang.CmdPackageCreateFlagRegistryOverride)
	devDeployFlags.StringVarP(&pkgConfig.CreateOpts.Flavor, "flavor", "f", v.GetString(common.VPkgCreateFlavor), lang.CmdPackageCreateFlagFlavor)

	devDeployFlags.StringToStringVar(&pkgConfig.PkgOpts.SetVariables, "deploy-set", v.GetStringMapString(common.VPkgDeploySet), lang.CmdPackageDeployFlagSet)

	// Always require adopt-existing-resources flag (no viper)
	devDeployFlags.BoolVar(&pkgConfig.DeployOpts.AdoptExistingResources, "adopt-existing-resources", false, lang.CmdPackageDeployFlagAdoptExistingResources)
	devDeployFlags.BoolVar(&pkgConfig.DeployOpts.SkipWebhooks, "skip-webhooks", v.GetBool(common.VPkgDeploySkipWebhooks), lang.CmdPackageDeployFlagSkipWebhooks)
	devDeployFlags.DurationVar(&pkgConfig.DeployOpts.Timeout, "timeout", v.GetDuration(common.VPkgDeployTimeout), lang.CmdPackageDeployFlagTimeout)

	devDeployFlags.IntVar(&pkgConfig.PkgOpts.Retries, "retries", v.GetInt(common.VPkgRetries), lang.CmdPackageFlagRetries)
	devDeployFlags.StringVar(&pkgConfig.PkgOpts.OptionalComponents, "components", v.GetString(common.VPkgDeployComponents), lang.CmdPackageDeployFlagComponents)

	devDeployFlags.BoolVar(&pkgConfig.CreateOpts.NoYOLO, "no-yolo", v.GetBool(common.VDevDeployNoYolo), lang.CmdDevDeployFlagNoYolo)
}

func bindDevGenerateFlags(_ *viper.Viper) {
	generateFlags := devGenerateCmd.Flags()

	generateFlags.StringVar(&pkgConfig.GenerateOpts.URL, "url", "", "URL to the source git repository")
	generateFlags.StringVar(&pkgConfig.GenerateOpts.Version, "version", "", "The Version of the chart to use")
	generateFlags.StringVar(&pkgConfig.GenerateOpts.GitPath, "gitPath", "", "Relative path to the chart in the git repository")
	generateFlags.StringVar(&pkgConfig.GenerateOpts.Output, "output-directory", "", "Output directory for the generated zarf.yaml")
	generateFlags.StringVar(&pkgConfig.FindImagesOpts.KubeVersionOverride, "kube-version", "", lang.CmdDevFlagKubeVersion)

	devGenerateCmd.MarkFlagRequired("url")
	devGenerateCmd.MarkFlagRequired("version")
	devGenerateCmd.MarkFlagRequired("output-directory")
}
