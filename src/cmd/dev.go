// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/pkg/helpers/v2"
	goyaml "github.com/goccy/go-yaml"
	"github.com/pterm/pterm"
	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/archive"
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/packager/load"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/pkg/utils"
)

var defaultRegistry = fmt.Sprintf("%s:%d", helpers.IPV4Localhost, state.ZarfInClusterContainerRegistryNodePort)

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
	cmd.AddCommand(newDevInspectManifestsCommand(v))
	cmd.AddCommand(newDevInspectValuesFilesCommand(v))
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
	cachePath, err := getCachePath(ctx)
	if err != nil {
		return err
	}
	loadOpts := load.DefinitionOptions{
		Flavor:           o.flavor,
		SetVariables:     o.setVariables,
		CachePath:        cachePath,
		IsInteractive:    true,
		SkipVersionCheck: true,
	}
	pkg, err := load.PackageDefinition(ctx, setBaseDirectory(args), loadOpts)
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

type devInspectManifestsOptions struct {
	flavor             string
	createSetVariables map[string]string
	deploySetVariables map[string]string
	kubeVersion        string
	outputWriter       io.Writer
}

func newDevInspectManifestsOptions() devInspectManifestsOptions {
	return devInspectManifestsOptions{
		outputWriter: OutputWriter,
	}
}

func newDevInspectManifestsCommand(v *viper.Viper) *cobra.Command {
	o := newDevInspectManifestsOptions()

	cmd := &cobra.Command{
		Use:   "manifests [ DIRECTORY ]",
		Args:  cobra.MaximumNArgs(1),
		Short: "Template and output all manifests and charts referenced by the package definition",
		RunE: func(cmd *cobra.Command, args []string) error {
			return o.run(cmd.Context(), args)
		},
	}

	cmd.Flags().StringVarP(&o.flavor, "flavor", "f", "", lang.CmdPackageCreateFlagFlavor)
	cmd.Flags().StringToStringVar(&o.createSetVariables, "create-set", v.GetStringMapString(VPkgCreateSet), lang.CmdPackageCreateFlagSet)
	cmd.Flags().StringToStringVar(&o.deploySetVariables, "deploy-set", v.GetStringMapString(VPkgDeploySet), lang.CmdPackageDeployFlagSet)
	cmd.Flags().StringVar(&o.kubeVersion, "kube-version", "", lang.CmdDevFlagKubeVersion)

	return cmd
}

func (o *devInspectManifestsOptions) run(ctx context.Context, args []string) error {
	v := getViper()
	o.createSetVariables = helpers.TransformAndMergeMap(
		v.GetStringMapString(VPkgCreateSet), o.createSetVariables, strings.ToUpper)
	o.deploySetVariables = helpers.TransformAndMergeMap(
		v.GetStringMapString(VPkgDeploySet), o.deploySetVariables, strings.ToUpper)
	cachePath, err := getCachePath(ctx)
	if err != nil {
		return err
	}
	opts := packager.InspectDefinitionResourcesOptions{
		CreateSetVariables: o.createSetVariables,
		DeploySetVariables: o.deploySetVariables,
		Flavor:             o.flavor,
		KubeVersion:        o.kubeVersion,
		CachePath:          cachePath,
		IsInteractive:      true,
	}
	resources, err := packager.InspectDefinitionResources(ctx, setBaseDirectory(args), opts)
	var lintErr *lint.LintError
	if errors.As(err, &lintErr) {
		PrintFindings(ctx, lintErr)
	}
	if err != nil {
		return err
	}
	resources = slices.DeleteFunc(resources, func(r packager.Resource) bool {
		return r.ResourceType == packager.ValuesFileResource
	})
	if len(resources) == 0 {
		return fmt.Errorf("0 manifests found")
	}
	for _, resource := range resources {
		fmt.Fprintf(o.outputWriter, "#type: %s\n", resource.ResourceType)
		// Helm charts already provide a comment on the source when templated
		if resource.ResourceType == packager.ManifestResource {
			fmt.Fprintf(o.outputWriter, "#source: %s\n", resource.Name)
		}
		fmt.Fprintf(o.outputWriter, "%s---\n", resource.Content)
	}
	return nil
}

type devInspectValuesFilesOptions struct {
	flavor             string
	createSetVariables map[string]string
	deploySetVariables map[string]string
	kubeVersion        string
	outputWriter       io.Writer
}

func newDevInspectValuesFilesOptions() devInspectValuesFilesOptions {
	return devInspectValuesFilesOptions{
		outputWriter: OutputWriter,
	}
}

func newDevInspectValuesFilesCommand(v *viper.Viper) *cobra.Command {
	o := newDevInspectValuesFilesOptions()

	cmd := &cobra.Command{
		Use:   "values-files [ DIRECTORY ]",
		Args:  cobra.MaximumNArgs(1),
		Short: "Creates, templates, and outputs the values-files to be sent to each chart",
		Long:  "Creates, templates, and outputs the values-files to be sent to each chart. Does not consider values files builtin to charts",
		RunE: func(cmd *cobra.Command, args []string) error {
			return o.run(cmd.Context(), args)
		},
	}

	cmd.Flags().StringVarP(&o.flavor, "flavor", "f", "", lang.CmdPackageCreateFlagFlavor)
	cmd.Flags().StringToStringVar(&o.createSetVariables, "create-set", v.GetStringMapString(VPkgCreateSet), lang.CmdPackageCreateFlagSet)
	cmd.Flags().StringToStringVar(&o.deploySetVariables, "deploy-set", v.GetStringMapString(VPkgDeploySet), lang.CmdPackageDeployFlagSet)
	cmd.Flags().StringVar(&o.kubeVersion, "kube-version", "", lang.CmdDevFlagKubeVersion)

	return cmd
}

func (o *devInspectValuesFilesOptions) run(ctx context.Context, args []string) error {
	v := getViper()
	o.createSetVariables = helpers.TransformAndMergeMap(
		v.GetStringMapString(VPkgCreateSet), o.createSetVariables, strings.ToUpper)
	o.deploySetVariables = helpers.TransformAndMergeMap(
		v.GetStringMapString(VPkgDeploySet), o.deploySetVariables, strings.ToUpper)
	cachePath, err := getCachePath(ctx)
	if err != nil {
		return err
	}
	opts := packager.InspectDefinitionResourcesOptions{
		CreateSetVariables: o.createSetVariables,
		DeploySetVariables: o.deploySetVariables,
		Flavor:             o.flavor,
		KubeVersion:        o.kubeVersion,
		CachePath:          cachePath,
		IsInteractive:      true,
	}
	resources, err := packager.InspectDefinitionResources(ctx, setBaseDirectory(args), opts)
	var lintErr *lint.LintError
	if errors.As(err, &lintErr) {
		PrintFindings(ctx, lintErr)
	}
	if err != nil {
		return err
	}
	resources = slices.DeleteFunc(resources, func(r packager.Resource) bool {
		return r.ResourceType != packager.ValuesFileResource
	})
	if len(resources) == 0 {
		return fmt.Errorf("0 values files found")
	}
	for _, resource := range resources {
		fmt.Fprintf(o.outputWriter, "# associated chart: %s\n", resource.Name)
		fmt.Fprintf(o.outputWriter, "%s---\n", resource.Content)
	}
	return nil
}

type devDeployOptions struct {
	createSetVariables     map[string]string
	deploySetVariables     map[string]string
	registryOverrides      []string
	flavor                 string
	registryURL            string
	adoptExistingResources bool
	timeout                time.Duration
	retries                int
	optionalComponents     string
	noYOLO                 bool
	ociConcurrency         int
	skipVersionCheck       bool
}

func newDevDeployCommand(v *viper.Viper) *cobra.Command {
	o := &devDeployOptions{}

	cmd := &cobra.Command{
		Use:   "deploy",
		Args:  cobra.MaximumNArgs(1),
		Short: lang.CmdDevDeployShort,
		Long:  lang.CmdDevDeployLong,
		RunE:  o.run,
	}

	cmd.Flags().StringToStringVar(&o.createSetVariables, "create-set", v.GetStringMapString(VPkgCreateSet), lang.CmdPackageCreateFlagSet)
	cmd.Flags().StringArrayVar(&o.registryOverrides, "registry-override", v.GetStringSlice(VPkgCreateRegistryOverride), lang.CmdPackageCreateFlagRegistryOverride)
	cmd.Flags().StringVarP(&o.flavor, "flavor", "f", v.GetString(VPkgCreateFlavor), lang.CmdPackageCreateFlagFlavor)

	cmd.Flags().StringVar(&o.registryURL, "registry-url", defaultRegistry, lang.CmdDevFlagRegistry)
	err := cmd.Flags().MarkHidden("registry-url")
	if err != nil {
		logger.Default().Debug("unable to mark dev-deploy flag as hidden", "error", err)
	}

	cmd.Flags().StringToStringVar(&o.deploySetVariables, "deploy-set", v.GetStringMapString(VPkgDeploySet), lang.CmdPackageDeployFlagSet)

	// Always require adopt-existing-resources flag (no viper)
	cmd.Flags().BoolVar(&o.adoptExistingResources, "adopt-existing-resources", false, lang.CmdPackageDeployFlagAdoptExistingResources)
	cmd.Flags().DurationVar(&o.timeout, "timeout", v.GetDuration(VPkgDeployTimeout), lang.CmdPackageDeployFlagTimeout)

	cmd.Flags().IntVar(&o.retries, "retries", v.GetInt(VPkgRetries), lang.CmdPackageFlagRetries)
	cmd.Flags().StringVar(&o.optionalComponents, "components", v.GetString(VPkgDeployComponents), lang.CmdPackageDeployFlagComponents)

	cmd.Flags().BoolVar(&o.noYOLO, "no-yolo", v.GetBool(VDevDeployNoYolo), lang.CmdDevDeployFlagNoYolo)

	cmd.Flags().IntVar(&o.ociConcurrency, "oci-concurrency", v.GetInt(VPkgOCIConcurrency), lang.CmdPackageFlagConcurrency)
	cmd.Flags().BoolVar(&o.skipVersionCheck, "skip-version-check", false, "Ignore version requirements when deploying the package")
	_ = cmd.Flags().MarkHidden("skip-version-check")

	return cmd
}

func (o *devDeployOptions) run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	baseDir := setBaseDirectory(args)

	v := getViper()
	o.createSetVariables = helpers.TransformAndMergeMap(
		v.GetStringMapString(VPkgCreateSet), o.createSetVariables, strings.ToUpper)

	o.deploySetVariables = helpers.TransformAndMergeMap(
		v.GetStringMapString(VPkgDeploySet), o.deploySetVariables, strings.ToUpper)

	cachePath, err := getCachePath(ctx)
	if err != nil {
		return err
	}
	overrides, err := parseRegistryOverrides(o.registryOverrides)
	if err != nil {
		return fmt.Errorf("error parsing registry override: %w", err)
	}

	err = packager.DevDeploy(ctx, baseDir, packager.DevDeployOptions{
		AirgapMode:         o.noYOLO,
		Flavor:             o.flavor,
		RegistryURL:        o.registryURL,
		RegistryOverrides:  overrides,
		CreateSetVariables: o.createSetVariables,
		DeploySetVariables: o.deploySetVariables,
		OptionalComponents: o.optionalComponents,
		Timeout:            o.timeout,
		Retries:            o.retries,
		OCIConcurrency:     o.ociConcurrency,
		RemoteOptions:      defaultRemoteOptions(),
		CachePath:          cachePath,
		SkipVersionCheck:   o.skipVersionCheck,
	})
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
	opts := packager.GenerateOptions{
		GitPath:     o.gitPath,
		KubeVersion: o.kubeVersion,
	}
	pkg, err := packager.Generate(cmd.Context(), name, o.url, o.version, opts)
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

	return os.WriteFile(generatedZarfYAMLPath, []byte(content), helpers.ReadAllWriteUser)
}

type devPatchGitOptions struct {
	gitPushUsername string
}

func newDevPatchGitCommand() *cobra.Command {
	o := &devPatchGitOptions{}

	cmd := &cobra.Command{
		Use:     "patch-git HOST FILE",
		Aliases: []string{"p"},
		Short:   lang.CmdDevPatchGitShort,
		Args:    cobra.ExactArgs(2),
		RunE:    o.run,
	}

	cmd.Flags().StringVar(&o.gitPushUsername, "git-account", state.ZarfGitPushUser, lang.CmdDevFlagGitAccount)

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
	// Perform git url transformation via regex
	text := string(content)

	processedText := transform.MutateGitURLsInText(l.Warn, host, text, o.gitPushUsername, false)

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
	ctx := cmd.Context()

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
		err = utils.DownloadToFile(ctx, fileName, downloadPath)
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

		decompressOpts := archive.DecompressOpts{
			Files: []string{extractedFile},
		}
		err = archive.Decompress(ctx, fileName, tmp, decompressOpts)
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

type devFindImagesOptions struct {
	repoHelmChartPath   string
	createSetVariables  map[string]string
	flavor              string
	deploySetVariables  map[string]string
	kubeVersionOverride string
	why                 string
	skipCosign          bool
	registryURL         string
	update              bool
}

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

	cmd.Flags().StringVarP(&o.repoHelmChartPath, "repo-chart-path", "p", "", lang.CmdDevFlagRepoChartPath)
	// use the package create config for this and reset it here to avoid overwriting the config.CreateOptions.SetVariables
	cmd.Flags().StringToStringVar(&o.createSetVariables, "set", v.GetStringMapString(VPkgCreateSet), lang.CmdDevFlagSet)

	err := cmd.Flags().MarkDeprecated("set", "this field is replaced by create-set")
	if err != nil {
		logger.Default().Debug("unable to mark dev-find-images flag as set", "error", err)
	}
	err = cmd.Flags().MarkHidden("set")
	if err != nil {
		logger.Default().Debug("unable to mark dev-find-images flag as hidden", "error", err)
	}
	cmd.Flags().StringVarP(&o.flavor, "flavor", "f", v.GetString(VPkgCreateFlavor), lang.CmdPackageCreateFlagFlavor)
	cmd.Flags().StringToStringVar(&o.createSetVariables, "create-set", v.GetStringMapString(VPkgCreateSet), lang.CmdDevFlagSet)
	cmd.Flags().StringToStringVar(&o.deploySetVariables, "deploy-set", v.GetStringMapString(VPkgDeploySet), lang.CmdPackageDeployFlagSet)
	// allow for the override of the default helm KubeVersion
	cmd.Flags().StringVar(&o.kubeVersionOverride, "kube-version", "", lang.CmdDevFlagKubeVersion)
	// check which manifests are using this particular image
	cmd.Flags().StringVar(&o.why, "why", "", lang.CmdDevFlagFindImagesWhy)
	// skip searching cosign artifacts in find images
	cmd.Flags().BoolVar(&o.skipCosign, "skip-cosign", false, lang.CmdDevFlagFindImagesSkipCosign)
	// update images in zarf.yaml file
	cmd.Flags().BoolVarP(&o.update, "update", "u", false, lang.CmdDevFlagFindImagesUpdate)

	cmd.Flags().StringVar(&o.registryURL, "registry-url", defaultRegistry, lang.CmdDevFlagRegistry)

	return cmd
}

func (o *devFindImagesOptions) run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	baseDir := setBaseDirectory(args)

	v := getViper()

	o.createSetVariables = helpers.TransformAndMergeMap(
		v.GetStringMapString(VPkgCreateSet), o.createSetVariables, strings.ToUpper)
	o.deploySetVariables = helpers.TransformAndMergeMap(
		v.GetStringMapString(VPkgDeploySet), o.deploySetVariables, strings.ToUpper)

	cachePath, err := getCachePath(ctx)
	if err != nil {
		return err
	}

	findImagesOptions := packager.FindImagesOptions{
		RepoHelmChartPath:   o.repoHelmChartPath,
		RegistryURL:         o.registryURL,
		KubeVersionOverride: o.kubeVersionOverride,
		CreateSetVariables:  o.createSetVariables,
		DeploySetVariables:  o.deploySetVariables,
		Flavor:              o.flavor,
		Why:                 o.why,
		SkipCosign:          o.skipCosign,
		CachePath:           cachePath,
		IsInteractive:       true,
	}
	imagesScans, err := packager.FindImages(ctx, baseDir, findImagesOptions)
	var lintErr *lint.LintError
	if errors.As(err, &lintErr) {
		PrintFindings(ctx, lintErr)
	}
	if err != nil {
		return fmt.Errorf("unable to find images: %w", err)
	}

	if o.why != "" {
		var foundWhyResource bool
		for _, scan := range imagesScans {
			for _, whyResource := range scan.WhyResources {
				fmt.Printf("component: %s\n%s: %s\nresource:\n\n%s\n", scan.ComponentName,
					whyResource.ResourceType, whyResource.Name, whyResource.Content)
				foundWhyResource = true
			}
		}
		if !foundWhyResource {
			return fmt.Errorf("image %s not found in any charts or manifests", o.why)
		}
		return nil
	}

	componentDefinition := "\ncomponents:\n"
	for _, finding := range imagesScans {
		if len(finding.Matches)+len(finding.PotentialMatches)+len(finding.CosignArtifacts) > 0 {
			componentDefinition += fmt.Sprintf("  - name: %s\n    images:\n", finding.ComponentName)
		}
		if len(finding.Matches) > 0 {
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

	if o.update {
		if err := packager.UpdateImages(ctx, baseDir, imagesScans); err != nil {
			return fmt.Errorf("unable to create update: %w", err)
		}
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

	v := getViper()
	// TODO once other formats are fully deprecated move this to the global viper config
	viper.SupportedExts = []string{"toml", "yaml", "yml"}
	if err := v.SafeWriteConfigAs(fileName); err != nil {
		return fmt.Errorf("unable to write the config file %s, make sure the file doesn't already exist: %w", fileName, err)
	}
	return nil
}

type devLintOptions struct {
	setVariables map[string]string
	flavor       string
}

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

	cmd.Flags().StringToStringVar(&o.setVariables, "set", v.GetStringMapString(VPkgCreateSet), lang.CmdPackageCreateFlagSet)
	cmd.Flags().StringVarP(&o.flavor, "flavor", "f", v.GetString(VPkgCreateFlavor), lang.CmdPackageCreateFlagFlavor)

	return cmd
}

func (o *devLintOptions) run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	baseDir := setBaseDirectory(args)
	v := getViper()
	o.setVariables = helpers.TransformAndMergeMap(
		v.GetStringMapString(VPkgCreateSet), o.setVariables, strings.ToUpper)
	cachePath, err := getCachePath(ctx)
	if err != nil {
		return err
	}
	err = packager.Lint(ctx, baseDir, packager.LintOptions{
		Flavor:       o.flavor,
		SetVariables: o.setVariables,
		CachePath:    cachePath,
	})
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
