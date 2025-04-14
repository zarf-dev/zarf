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
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/pkg/helpers/v2"
	goyaml "github.com/goccy/go-yaml"
	"github.com/mholt/archiver/v3"
	"github.com/pterm/pterm"
	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/internal/packager2"
	layout2 "github.com/zarf-dev/zarf/src/internal/packager2/layout"
	"github.com/zarf-dev/zarf/src/pkg/layout"
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/pkg/logger"
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

	v := getViper()

	cmd.AddCommand(newDevDeployCommand(v))
	cmd.AddCommand(newDevGenerateCommand())
	cmd.AddCommand(newDevPatchGitCommand())
	cmd.AddCommand(newDevSha256SumCommand())
	cmd.AddCommand(newDevInspectCommand(v))
	cmd.AddCommand(newDevFindImagesCommand(v))
	cmd.AddCommand(newDevGenerateConfigCommand())
	cmd.AddCommand(newDevLintCommand(v))

	return cmd
}

func newDevInspectCommand(v *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Commands to get information about a Zarf package using a `zarf.yaml`",
	}

	cmd.AddCommand(newDevInspectDefinitionCommand(v))
	return cmd
}

type devInspectDefinitionOptions struct {
	flavor       string
	setVariables map[string]string
}

func newDevInspectDefinitionCommand(v *viper.Viper) *cobra.Command {
	o := &devInspectDefinitionOptions{}

	cmd := &cobra.Command{
		Use:   "definition [ DIRECTORY ]",
		Args:  cobra.MaximumNArgs(1),
		Short: "Displays the fully rendered package definition",
		Long:  "Displays the 'zarf.yaml' definition of a Zarf after package templating, flavors, and component imports are applied",
		RunE:  o.run,
	}

	cmd.Flags().StringVarP(&o.flavor, "flavor", "f", "", lang.CmdPackageCreateFlagFlavor)
	cmd.Flags().StringToStringVar(&o.setVariables, "set", v.GetStringMapString(VPkgCreateSet), lang.CmdPackageCreateFlagSet)

	return cmd
}

func (o *devInspectDefinitionOptions) run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	v := getViper()
	o.setVariables = helpers.TransformAndMergeMap(
		v.GetStringMapString(VPkgCreateSet), o.setVariables, strings.ToUpper)
	pkg, err := layout2.LoadPackageDefinition(ctx, setBaseDirectory(args), o.flavor, o.setVariables)
	if err != nil {
		return err
	}
	pkg.Build = v1alpha1.ZarfBuildData{}
	err = utils.ColorPrintYAML(pkg, nil, false)
	if err != nil {
		return err
	}
	return nil
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
	cmd.Flags().StringToStringVar(&pkgConfig.CreateOpts.SetVariables, "create-set", v.GetStringMapString(VPkgCreateSet), lang.CmdPackageCreateFlagSet)
	cmd.Flags().StringToStringVar(&pkgConfig.CreateOpts.RegistryOverrides, "registry-override", v.GetStringMapString(VPkgCreateRegistryOverride), lang.CmdPackageCreateFlagRegistryOverride)
	cmd.Flags().StringVarP(&pkgConfig.CreateOpts.Flavor, "flavor", "f", v.GetString(VPkgCreateFlavor), lang.CmdPackageCreateFlagFlavor)

	cmd.Flags().StringVar(&pkgConfig.DeployOpts.RegistryURL, "registry-url", defaultRegistry, lang.CmdDevFlagRegistry)
	err := cmd.Flags().MarkHidden("registry-url")
	if err != nil {
		logger.Default().Debug("unable to mark dev-deploy flag as hidden", "error", err)
	}

	cmd.Flags().StringToStringVar(&pkgConfig.PkgOpts.SetVariables, "deploy-set", v.GetStringMapString(VPkgDeploySet), lang.CmdPackageDeployFlagSet)

	// Always require adopt-existing-resources flag (no viper)
	cmd.Flags().BoolVar(&pkgConfig.DeployOpts.AdoptExistingResources, "adopt-existing-resources", false, lang.CmdPackageDeployFlagAdoptExistingResources)
	cmd.Flags().DurationVar(&pkgConfig.DeployOpts.Timeout, "timeout", v.GetDuration(VPkgDeployTimeout), lang.CmdPackageDeployFlagTimeout)

	cmd.Flags().IntVar(&pkgConfig.PkgOpts.Retries, "retries", v.GetInt(VPkgRetries), lang.CmdPackageFlagRetries)
	cmd.Flags().StringVar(&pkgConfig.PkgOpts.OptionalComponents, "components", v.GetString(VPkgDeployComponents), lang.CmdPackageDeployFlagComponents)

	cmd.Flags().BoolVar(&pkgConfig.CreateOpts.NoYOLO, "no-yolo", v.GetBool(VDevDeployNoYolo), lang.CmdDevDeployFlagNoYolo)

	return cmd
}

func (o *devDeployOptions) run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	pkgConfig.CreateOpts.BaseDir = setBaseDirectory(args)

	v := getViper()
	pkgConfig.CreateOpts.SetVariables = helpers.TransformAndMergeMap(
		v.GetStringMapString(VPkgCreateSet), pkgConfig.CreateOpts.SetVariables, strings.ToUpper)

	pkgConfig.PkgOpts.SetVariables = helpers.TransformAndMergeMap(
		v.GetStringMapString(VPkgDeploySet), pkgConfig.PkgOpts.SetVariables, strings.ToUpper)

	pkgClient, err := packager.New(&pkgConfig, packager.WithContext(ctx))
	if err != nil {
		return err
	}
	defer pkgClient.ClearTempPaths()

	err = pkgClient.DevDeploy(ctx)
	var lintErr *lint.LintError
	if errors.As(err, &lintErr) {
		PrintFindings(ctx, lintErr)
	}
	if err != nil {
		return fmt.Errorf("failed to dev deploy: %w", err)
	}

	return nil
}

type devGenerateOptions struct {
	url         string
	version     string
	gitPath     string
	output      string
	kubeVersion string
}

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

	cmd.Flags().StringVar(&o.url, "url", "", "URL to the source git repository")
	cmd.MarkFlagRequired("url")
	cmd.Flags().StringVar(&o.version, "version", "", "The Version of the chart to use")
	cmd.MarkFlagRequired("version")
	cmd.Flags().StringVar(&o.gitPath, "gitPath", "", "Relative path to the chart in the git repository")
	cmd.Flags().StringVar(&o.output, "output-directory", "", "Output directory for the generated zarf.yaml")
	cmd.MarkFlagRequired("output-directory")
	cmd.Flags().StringVar(&o.kubeVersion, "kube-version", "", lang.CmdDevFlagKubeVersion)

	return cmd
}

func (o *devGenerateOptions) run(cmd *cobra.Command, args []string) (err error) {
	l := logger.From(cmd.Context())
	start := time.Now()
	name := args[0]
	generatedZarfYAMLPath := filepath.Join(o.output, layout.ZarfYAML)

	if !helpers.InvalidPath(generatedZarfYAMLPath) {
		prefixed := filepath.Join(o.output, fmt.Sprintf("%s-%s", name, layout.ZarfYAML))
		l.Warn("using a prefixed name since zarf.yaml already exists in the output directory",
			"output-directory", o.output,
			"name", prefixed)
		generatedZarfYAMLPath = prefixed
		if !helpers.InvalidPath(generatedZarfYAMLPath) {
			return fmt.Errorf("unable to generate package, %s already exists", generatedZarfYAMLPath)
		}
	}
	l.Info("generating package", "name", name, "path", generatedZarfYAMLPath)
	opts := &packager2.GenerateOptions{
		PackageName: name,
		Version:     o.version,
		URL:         o.url,
		GitPath:     o.gitPath,
		KubeVersion: o.kubeVersion,
	}
	pkg, err := packager2.Generate(cmd.Context(), opts)
	if err != nil {
		return err
	}

	if err := helpers.CreateDirectory(o.output, helpers.ReadExecuteAllWriteUser); err != nil {
		return err
	}

	b, err := goyaml.MarshalWithOptions(pkg, goyaml.IndentSequence(true), goyaml.UseSingleQuote(false))
	if err != nil {
		return err
	}

	schemaComment := fmt.Sprintf("# yaml-language-server: $schema=https://raw.githubusercontent.com/%s/%s/zarf.schema.json", config.GithubProject, config.CLIVersion)
	content := schemaComment + "\n" + string(b)

	// lets space things out a bit
	content = strings.Replace(content, "kind:\n", "\nkind:\n", 1)
	content = strings.Replace(content, "metadata:\n", "\nmetadata:\n", 1)
	content = strings.Replace(content, "components:\n", "\ncomponents:\n", 1)

	l.Debug("generated package", "name", name, "path", generatedZarfYAMLPath, "duration", time.Since(start))

	return os.WriteFile(generatedZarfYAMLPath, []byte(content), helpers.ReadAllWriteUser)
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

func (o *devPatchGitOptions) run(cmd *cobra.Command, args []string) error {
	l := logger.From(cmd.Context())
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

	processedText := transform.MutateGitURLsInText(l.Warn, gitServer.Address, text, gitServer.PushUsername)

	// Print the differences
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
	cmd.Flags().StringToStringVar(&pkgConfig.CreateOpts.SetVariables, "set", v.GetStringMapString(VPkgCreateSet), lang.CmdDevFlagSet)

	err := cmd.Flags().MarkDeprecated("set", "this field is replaced by create-set")
	if err != nil {
		logger.Default().Debug("unable to mark dev-find-images flag as set", "error", err)
	}
	err = cmd.Flags().MarkHidden("set")
	if err != nil {
		logger.Default().Debug("unable to mark dev-find-images flag as hidden", "error", err)
	}
	cmd.Flags().StringVarP(&pkgConfig.CreateOpts.Flavor, "flavor", "f", v.GetString(VPkgCreateFlavor), lang.CmdPackageCreateFlagFlavor)
	cmd.Flags().StringToStringVar(&pkgConfig.CreateOpts.SetVariables, "create-set", v.GetStringMapString(VPkgCreateSet), lang.CmdDevFlagSet)
	cmd.Flags().StringToStringVar(&pkgConfig.PkgOpts.SetVariables, "deploy-set", v.GetStringMapString(VPkgDeploySet), lang.CmdPackageDeployFlagSet)
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
	baseDir := setBaseDirectory(args)

	v := getViper()

	pkgConfig.CreateOpts.SetVariables = helpers.TransformAndMergeMap(
		v.GetStringMapString(VPkgCreateSet), pkgConfig.CreateOpts.SetVariables, strings.ToUpper)
	pkgConfig.PkgOpts.SetVariables = helpers.TransformAndMergeMap(
		v.GetStringMapString(VPkgDeploySet), pkgConfig.PkgOpts.SetVariables, strings.ToUpper)

	findImagesOptions := packager2.FindImagesOptions{
		RepoHelmChartPath:   pkgConfig.FindImagesOpts.RepoHelmChartPath,
		RegistryURL:         pkgConfig.FindImagesOpts.RegistryURL,
		KubeVersionOverride: pkgConfig.FindImagesOpts.KubeVersionOverride,
		CreateSetVariables:  pkgConfig.CreateOpts.SetVariables,
		DeploySetVariables:  pkgConfig.PkgOpts.SetVariables,
		Flavor:              pkgConfig.CreateOpts.Flavor,
		Why:                 pkgConfig.FindImagesOpts.Why,
		SkipCosign:          pkgConfig.FindImagesOpts.SkipCosign,
	}
	results, err := packager2.FindImages(ctx, baseDir, findImagesOptions)
	var lintErr *lint.LintError
	if errors.As(err, &lintErr) {
		PrintFindings(ctx, lintErr)
	}
	if err != nil {
		return fmt.Errorf("unable to find images: %w", err)
	}

	if pkgConfig.FindImagesOpts.Why != "" {
		var foundWhyResource bool
		for _, scan := range results.ComponentImageScans {
			for _, whyResource := range scan.WhyResources {
				fmt.Printf("component: %s\n%s: %s\nresource:\n\n%s\n", scan.ComponentName,
					whyResource.ResourceType, whyResource.Name, whyResource.Content)
				foundWhyResource = true
			}
		}
		if !foundWhyResource {
			return fmt.Errorf("image %s not found in any charts or manifests", pkgConfig.FindImagesOpts.Why)
		}
		return nil
	}

	componentDefinition := "\ncomponents:\n"
	for _, finding := range results.ComponentImageScans {
		if len(finding.Matches) > 0 {
			componentDefinition += fmt.Sprintf("  - name: %s\n    images:\n", finding.ComponentName)
			for _, image := range finding.Matches {
				componentDefinition += fmt.Sprintf("      - %s\n", image)
			}
		}
		if len(finding.PotentialMatches) > 0 {
			componentDefinition += fmt.Sprintf("      # Possible images - %s\n", finding.ComponentName)
			for _, image := range finding.PotentialMatches {
				componentDefinition += fmt.Sprintf("      - %s\n", image)
			}
		}
		if len(finding.CosignArtifacts) > 0 {
			componentDefinition += fmt.Sprintf("      # Cosign artifacts for images - %s\n", finding.ComponentName)
			for _, cosignArtifact := range finding.CosignArtifacts {
				componentDefinition += fmt.Sprintf("      - %s\n", cosignArtifact)
			}
		}
	}
	fmt.Println(componentDefinition)
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

	v := getViper()
	// TODO once other formats are fully deprecated move this to the global viper config
	viper.SupportedExts = []string{"toml", "yaml", "yml"}
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

	cmd.Flags().StringToStringVar(&pkgConfig.CreateOpts.SetVariables, "set", v.GetStringMapString(VPkgCreateSet), lang.CmdPackageCreateFlagSet)
	cmd.Flags().StringVarP(&pkgConfig.CreateOpts.Flavor, "flavor", "f", v.GetString(VPkgCreateFlavor), lang.CmdPackageCreateFlagFlavor)

	return cmd
}

func (o *devLintOptions) run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	config.CommonOptions.Confirm = true
	pkgConfig.CreateOpts.BaseDir = setBaseDirectory(args)
	v := getViper()
	pkgConfig.CreateOpts.SetVariables = helpers.TransformAndMergeMap(
		v.GetStringMapString(VPkgCreateSet), pkgConfig.CreateOpts.SetVariables, strings.ToUpper)

	err := lint.Validate(ctx, pkgConfig.CreateOpts.BaseDir, pkgConfig.CreateOpts.Flavor, pkgConfig.CreateOpts.SetVariables)
	var lintErr *lint.LintError
	if errors.As(err, &lintErr) {
		PrintFindings(ctx, lintErr)
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
