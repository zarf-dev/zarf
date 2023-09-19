//go:build !alt_language

// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package lang contains the language strings for english used by Zarf
// Alternative languages can be created by duplicating this file and changing the build tag to "//go:build alt_language && <language>".
package lang

import "errors"

// All language strings should be in the form of a constant
// The constants should be grouped by the top level package they are used in (or common)
// The format should be <PathName><Err/Info><ShortDescription>
// Debug messages will not be a part of the language strings since they are not intended to be user facing
// Include sprintf formatting directives in the string if needed.
const (
	ErrLoadingConfig       = "failed to load config: %w"
	ErrLoadState           = "Failed to load the Zarf State from the Kubernetes cluster."
	ErrSaveState           = "Failed to save the Zarf State to the Kubernetes cluster."
	ErrLoadPackageSecret   = "Failed to load %s's secret from the Kubernetes cluster"
	ErrMarshal             = "failed to marshal file: %w"
	ErrNoClusterConnection = "Failed to connect to the Kubernetes cluster."
	ErrTunnelFailed        = "Failed to create a tunnel to the Kubernetes cluster."
	ErrUnmarshal           = "failed to unmarshal file: %w"
	ErrWritingFile         = "failed to write file %s: %s"
	ErrDownloading         = "failed to download %s: %s"
	ErrCreatingDir         = "failed to create directory %s: %s"
	ErrRemoveFile          = "failed to remove file %s: %s"
	ErrUnarchive           = "failed to unarchive %s: %s"
	ErrConfirmCancel       = "confirm selection canceled: %s"
	ErrFileExtract         = "failed to extract filename %s from archive %s: %s"
	ErrFileNameExtract     = "failed to extract filename from URL %s: %s"
)

// Zarf CLI commands.
const (
	// common command language
	CmdConfirmProvided = "Confirm flag specified, continuing without prompting."
	CmdConfirmContinue = "Continue with these changes?"

	// root zarf command
	RootCmdShort = "DevSecOps for Airgap"
	RootCmdLong  = "Zarf eliminates the complexity of air gap software delivery for Kubernetes clusters and cloud native workloads\n" +
		"using a declarative packaging strategy to support DevSecOps in offline and semi-connected environments."

	RootCmdFlagLogLevel    = "Log level when running Zarf. Valid options are: warn, info, debug, trace"
	RootCmdFlagArch        = "Architecture for OCI images and Zarf packages"
	RootCmdFlagSkipLogFile = "Disable log file creation"
	RootCmdFlagNoProgress  = "Disable fancy UI progress bars, spinners, logos, etc"
	RootCmdFlagNoColor     = "Disable colors in output"
	RootCmdFlagCachePath   = "Specify the location of the Zarf cache directory"
	RootCmdFlagTempDir     = "Specify the temporary directory to use for intermediate files"
	RootCmdFlagInsecure    = "Allow access to insecure registries and disable other recommended security enforcements such as package checksum and signature validation. This flag should only be used if you have a specific reason and accept the reduced security posture."

	RootCmdDeprecatedDeploy = "Deprecated: Please use \"zarf package deploy %s\" to deploy this package.  This warning will be removed in Zarf v1.0.0."
	RootCmdDeprecatedCreate = "Deprecated: Please use \"zarf package create\" to create this package.  This warning will be removed in Zarf v1.0.0."

	RootCmdErrInvalidLogLevel = "Invalid log level. Valid options are: warn, info, debug, trace."

	// zarf connect
	CmdConnectShort = "Accesses services or pods deployed in the cluster"
	CmdConnectLong  = "Uses a k8s port-forward to connect to resources within the cluster referenced by your kube-context.\n" +
		"Three default options for this command are <REGISTRY|LOGGING|GIT>. These will connect to the Zarf created resources " +
		"(assuming they were selected when performing the `zarf init` command).\n\n" +
		"Packages can provide service manifests that define their own shortcut connection options. These options will be " +
		"printed to the terminal when the package finishes deploying.\n If you don't remember what connection shortcuts your deployed " +
		"package offers, you can search your cluster for services that have the 'zarf.dev/connect-name' label. The value of that label is " +
		"the name you will pass into the 'zarf connect' command.\n\n" +
		"Even if the packages you deploy don't define their own shortcut connection options, you can use the command flags " +
		"to connect into specific resources. You can read the command flag descriptions below to get a better idea how to connect " +
		"to whatever resource you are trying to connect to."

	// zarf connect list
	CmdConnectListShort = "Lists all available connection shortcuts"

	CmdConnectFlagName       = "Specify the resource name.  E.g. name=unicorns or name=unicorn-pod-7448499f4d-b5bk6"
	CmdConnectFlagNamespace  = "Specify the namespace.  E.g. namespace=default"
	CmdConnectFlagType       = "Specify the resource type.  E.g. type=svc or type=pod"
	CmdConnectFlagLocalPort  = "(Optional, autogenerated if not provided) Specify the local port to bind to.  E.g. local-port=42000"
	CmdConnectFlagRemotePort = "Specify the remote port of the resource to bind to.  E.g. remote-port=8080"
	CmdConnectFlagCliOnly    = "Disable browser auto-open"

	// zarf destroy
	CmdDestroyShort = "Tears down Zarf and removes its components from the environment"
	CmdDestroyLong  = "Tear down Zarf.\n\n" +
		"Deletes everything in the 'zarf' namespace within your connected k8s cluster.\n\n" +
		"If Zarf deployed your k8s cluster, this command will also tear your cluster down by " +
		"searching through /opt/zarf for any scripts that start with 'zarf-clean-' and executing them. " +
		"Since this is a cleanup operation, Zarf will not stop the teardown if one of the scripts produce " +
		"an error.\n\n" +
		"If Zarf did not deploy your k8s cluster, this command will delete the Zarf namespace, delete secrets " +
		"and labels that only Zarf cares about, and optionally uninstall components that Zarf deployed onto " +
		"the cluster. Since this is a cleanup operation, Zarf will not stop the uninstalls if one of the " +
		"resources produce an error while being deleted."

	CmdDestroyFlagConfirm          = "REQUIRED. Confirm the destroy action to prevent accidental deletions"
	CmdDestroyFlagRemoveComponents = "Also remove any installed components outside the zarf namespace"

	CmdDestroyErrNoScriptPath           = "Unable to find the folder (%s) which has the scripts to cleanup the cluster. Please double-check you have the right kube-context"
	CmdDestroyErrScriptPermissionDenied = "Received 'permission denied' when trying to execute the script (%s). Please double-check you have the correct kube-context."

	// zarf init
	CmdInitShort = "Prepares a k8s cluster for the deployment of Zarf packages"
	CmdInitLong  = "Injects a docker registry as well as other optional useful things (such as a git server " +
		"and a logging stack) into a k8s cluster under the 'zarf' namespace " +
		"to support future application deployments.\n" +
		"If you do not have a k8s cluster already configured, this command will give you " +
		"the ability to install a cluster locally.\n\n" +
		"This command looks for a zarf-init package in the local directory that the command was executed " +
		"from. If no package is found in the local directory and the Zarf CLI exists somewhere outside of " +
		"the current directory, Zarf will failover and attempt to find a zarf-init package in the directory " +
		"that the Zarf binary is located in.\n\n\n\n"

	CmdInitExample = `
	# Initializing without any optional components:
	zarf init

	# Initializing w/ Zarfs internal git server:
	zarf init --components=git-server

	# Initializing w/ Zarfs internal git server and PLG stack:
	zarf init --components=git-server,logging

	# Initializing w/ an internal registry but with a different nodeport:
	zarf init --nodeport=30333

	# Initializing w/ an external registry:
	zarf init --registry-push-password={PASSWORD} --registry-push-username={USERNAME} --registry-url={URL}

	# Initializing w/ an external git server:
	zarf init --git-push-password={PASSWORD} --git-push-username={USERNAME} --git-url={URL}

	# Initializing w/ an external artifact server:
	zarf init --artifact-push-password={PASSWORD} --artifact-push-username={USERNAME} --artifact-url={URL}

	# NOTE: Not specifying a pull username/password will use the push user for pulling as well.
`

	CmdInitErrFlags             = "Invalid command flags were provided."
	CmdInitErrDownload          = "failed to download the init package: %s"
	CmdInitErrValidateGit       = "the 'git-push-username' and 'git-push-password' flags must be provided if the 'git-url' flag is provided"
	CmdInitErrValidateRegistry  = "the 'registry-push-username' and 'registry-push-password' flags must be provided if the 'registry-url' flag is provided"
	CmdInitErrValidateArtifact  = "the 'artifact-push-username' and 'artifact-push-token' flags must be provided if the 'artifact-url' flag is provided"
	CmdInitErrUnableCreateCache = "Unable to create the cache directory: %s"

	CmdInitPullAsk       = "It seems the init package could not be found locally, but can be pulled from oci://%s"
	CmdInitPullNote      = "Note: This will require an internet connection."
	CmdInitPullConfirm   = "Do you want to pull this init package?"
	CmdInitPullErrManual = "pull the init package manually and place it in the current working directory"

	CmdInitFlagSet = "Specify deployment variables to set on the command line (KEY=value)"

	CmdInitFlagConfirm      = "Confirms package deployment without prompting. ONLY use with packages you trust. Skips prompts to review SBOM, configure variables, select optional components and review potential breaking changes."
	CmdInitFlagComponents   = "Specify which optional components to install.  E.g. --components=git-server,logging"
	CmdInitFlagStorageClass = "Specify the storage class to use for the registry and git server.  E.g. --storage-class=standard"

	CmdInitFlagGitURL      = "External git server url to use for this Zarf cluster"
	CmdInitFlagGitPushUser = "Username to access to the git server Zarf is configured to use. User must be able to create repositories via 'git push'"
	CmdInitFlagGitPushPass = "Password for the push-user to access the git server"
	CmdInitFlagGitPullUser = "Username for pull-only access to the git server"
	CmdInitFlagGitPullPass = "Password for the pull-only user to access the git server"

	CmdInitFlagRegURL      = "External registry url address to use for this Zarf cluster"
	CmdInitFlagRegNodePort = "Nodeport to access a registry internal to the k8s cluster. Between [30000-32767]"
	CmdInitFlagRegPushUser = "Username to access to the registry Zarf is configured to use"
	CmdInitFlagRegPushPass = "Password for the push-user to connect to the registry"
	CmdInitFlagRegPullUser = "Username for pull-only access to the registry"
	CmdInitFlagRegPullPass = "Password for the pull-only user to access the registry"
	CmdInitFlagRegSecret   = "Registry secret value"

	CmdInitFlagArtifactURL       = "[alpha] External artifact registry url to use for this Zarf cluster"
	CmdInitFlagArtifactPushUser  = "[alpha] Username to access to the artifact registry Zarf is configured to use. User must be able to upload package artifacts."
	CmdInitFlagArtifactPushToken = "[alpha] API Token for the push-user to access the artifact registry"

	// zarf internal
	CmdInternalShort = "Internal tools used by zarf"

	CmdInternalAgentShort = "Runs the zarf agent"
	CmdInternalAgentLong  = "NOTE: This command is a hidden command and generally shouldn't be run by a human.\n" +
		"This command starts up a http webhook that Zarf deployments use to mutate pods to conform " +
		"with the Zarf container registry and Gitea server URLs."

	CmdInternalProxyShort = "[alpha] Runs the zarf agent http proxy"
	CmdInternalProxyLong  = "[alpha] NOTE: This command is a hidden command and generally shouldn't be run by a human.\n" +
		"This command starts up a http proxy that can be used by running pods to transform queries " +
		"that conform to Gitea / Gitlab repository and package URLs in the airgap."

	CmdInternalGenerateCliDocsShort   = "Creates auto-generated markdown of all the commands for the CLI"
	CmdInternalGenerateCliDocsSuccess = "Successfully created the CLI documentation"
	CmdInternalGenerateCliDocsErr     = "Unable to generate the CLI documentation: %s"

	CmdInternalConfigSchemaShort = "Generates a JSON schema for the zarf.yaml configuration"
	CmdInternalConfigSchemaErr   = "Unable to generate the zarf config schema"

	CmdInternalAPISchemaShort       = "Generates a JSON schema from the API types"
	CmdInternalAPISchemaGenerateErr = "Unable to generate the zarf api schema"

	CmdInternalCreateReadOnlyGiteaUserShort = "Creates a read-only user in Gitea"
	CmdInternalCreateReadOnlyGiteaUserLong  = "Creates a read-only user in Gitea by using the Gitea API. " +
		"This is called internally by the supported Gitea package component."
	CmdInternalCreateReadOnlyGiteaUserErr = "Unable to create a read-only user in the Gitea service."

	CmdInternalArtifactRegistryGiteaTokenShort = "Creates an artifact registry token for Gitea"
	CmdInternalArtifactRegistryGiteaTokenLong  = "Creates an artifact registry token in Gitea using the Gitea API. " +
		"This is called internally by the supported Gitea package component."
	CmdInternalArtifactRegistryGiteaTokenErr = "Unable to create an artifact registry token for the Gitea service."

	CmdInternalUIShort = "[beta] Launches the Zarf Web UI"
	CmdInternalUILong  = "[beta] This command launches the Zarf deployment Web UI to connect to clusters and deploy packages" +
		"using a Web GUI instead of the CLI."

	CmdInternalIsValidHostnameShort = "Checks if the current machine's hostname is RFC1123 compliant"
	CmdInternalIsValidHostnameErr   = "The hostname '%s' is not valid. Ensure the hostname meets RFC1123 requirements https://www.rfc-editor.org/rfc/rfc1123.html."

	CmdInternalCrc32Short = "Generates a decimal CRC32 for the given text"

	// zarf package
	CmdPackageShort           = "Zarf package commands for creating, deploying, and inspecting packages"
	CmdPackageFlagConcurrency = "Number of concurrent layer operations to perform when interacting with a remote package."

	CmdPackageCreateShort = "Creates a Zarf package from a given directory or the current directory"
	CmdPackageCreateLong  = "Builds an archive of resources and dependencies defined by the 'zarf.yaml' in the specified directory.\n" +
		"Private registries and repositories are accessed via credentials in your local '~/.docker/config.json', " +
		"'~/.git-credentials' and '~/.netrc'.\n"

	CmdPackageDeployShort = "Deploys a Zarf package from a local file or URL (runs offline)"
	CmdPackageDeployLong  = "Unpacks resources and dependencies from a Zarf package archive and deploys them onto the target system.\n" +
		"Kubernetes clusters are accessed via credentials in your current kubecontext defined in '~/.kube/config'"

	CmdPackageMirrorShort = "Mirrors a Zarf package's internal resources to specified image registries and git repositories"
	CmdPackageMirrorLong  = "Unpacks resources and dependencies from a Zarf package archive and mirrors them into the specified \n" +
		"image registries and git repositories within the target environment"

	CmdPackageInspectShort = "Displays the definition of a Zarf package (runs offline)"
	CmdPackageInspectLong  = "Displays the 'zarf.yaml' definition for the specified package and optionally allows SBOMs to be viewed"

	CmdPackageListShort         = "Lists out all of the packages that have been deployed to the cluster (runs offline)"
	CmdPackageListNoPackageWarn = "Unable to get the packages deployed to the cluster"
	CmdPackageListUnmarshalErr  = "Unable to read all of the packages deployed to the cluster"

	CmdPackageCreateFlagConfirm            = "Confirm package creation without prompting"
	CmdPackageCreateFlagSet                = "Specify package variables to set on the command line (KEY=value)"
	CmdPackageCreateFlagOutput             = "Specify the output (either a directory or an oci:// URL) for the created Zarf package"
	CmdPackageCreateFlagSbom               = "View SBOM contents after creating the package"
	CmdPackageCreateFlagSbomOut            = "Specify an output directory for the SBOMs from the created Zarf package"
	CmdPackageCreateFlagSkipSbom           = "Skip generating SBOM for this package"
	CmdPackageCreateFlagMaxPackageSize     = "Specify the maximum size of the package in megabytes, packages larger than this will be split into multiple parts. Use 0 to disable splitting."
	CmdPackageCreateFlagSigningKey         = "Path to private key file for signing packages"
	CmdPackageCreateFlagSigningKeyPassword = "Password to the private key file used for signing packages"
	CmdPackageCreateFlagDifferential       = "[beta] Build a package that only contains the differential changes from local resources and differing remote resources from the specified previously built package"
	CmdPackageCreateFlagRegistryOverride   = "Specify a map of domains to override on package create when pulling images (e.g. --registry-override docker.io=dockerio-reg.enterprise.intranet)"
	CmdPackageCreateCleanPathErr           = "Invalid characters in Zarf cache path, defaulting to %s"
	CmdPackageCreateErr                    = "Failed to create package: %s"

	CmdPackageDeployFlagConfirm                        = "Confirms package deployment without prompting. ONLY use with packages you trust. Skips prompts to review SBOM, configure variables, select optional components and review potential breaking changes."
	CmdPackageDeployFlagAdoptExistingResources         = "Adopts any pre-existing K8s resources into the Helm charts managed by Zarf. ONLY use when you have existing deployments you want Zarf to takeover."
	CmdPackageDeployFlagSet                            = "Specify deployment variables to set on the command line (KEY=value)"
	CmdPackageDeployFlagComponents                     = "Comma-separated list of components to install.  Adding this flag will skip the init prompts for which components to install"
	CmdPackageDeployFlagShasum                         = "Shasum of the package to deploy. Required if deploying a remote package and \"--insecure\" is not provided"
	CmdPackageDeployFlagSget                           = "[Deprecated] Path to public sget key file for remote packages signed via cosign. This flag will be removed in v1.0.0 please use the --key flag instead."
	CmdPackageDeployFlagPublicKey                      = "Path to public key file for validating signed packages"
	CmdPackageDeployValidateArchitectureErr            = "this package architecture is %s, but the target cluster has the %s architecture. These architectures must be the same"
	CmdPackageDeployValidateLastNonBreakingVersionWarn = "the version of this Zarf binary '%s' is less than the LastNonBreakingVersion of '%s'. You may need to upgrade your Zarf version to at least '%s' to deploy this package"
	CmdPackageDeployInvalidCLIVersionWarn              = "CLIVersion is set to '%s' which can cause issues with package creation and deployment. To avoid such issues, please set the value to the valid semantic version for this version of Zarf."
	CmdPackageDeployErr                                = "Failed to deploy package: %s"

	CmdPackageMirrorFlagComponents = "Comma-separated list of components to mirror.  This list will be respected regardless of a component's 'required' status."
	CmdPackageMirrorFlagNoChecksum = "Turns off the addition of a checksum to image tags (as would be used by the Zarf Agent) while mirroring images."

	CmdPackageInspectFlagSbom      = "View SBOM contents while inspecting the package"
	CmdPackageInspectFlagSbomOut   = "Specify an output directory for the SBOMs from the inspected Zarf package"
	CmdPackageInspectFlagValidate  = "Validate any checksums and signatures while inspecting the package"
	CmdPackageInspectFlagPublicKey = "Path to a public key file that will be used to validate a signed package"
	CmdPackageInspectErr           = "Failed to inspect package: %s"

	CmdPackageRemoveShort          = "Removes a Zarf package that has been deployed already (runs offline)"
	CmdPackageRemoveFlagConfirm    = "REQUIRED. Confirm the removal action to prevent accidental deletions"
	CmdPackageRemoveFlagComponents = "Comma-separated list of components to uninstall"
	CmdPackageRemoveTarballErr     = "Invalid tarball path provided"
	CmdPackageRemoveExtractErr     = "Unable to extract the package contents"
	CmdPackageRemoveErr            = "Unable to remove the package with an error of: %s"

	CmdPackageRegistryPrefixErr = "Registry must be prefixed with 'oci://'"

	CmdPackagePublishShort   = "Publishes a Zarf package to a remote registry"
	CmdPackagePublishExample = `
	# Publish a package to a remote registry
	zarf package publish my-package.tar oci://my-registry.com/my-namespace

	# Publish a skeleton package to a remote registry
	zarf package publish ./path/to/dir oci://my-registry.com/my-namespace
`
	CmdPackagePublishFlagSigningKey         = "Path to private key file for signing packages"
	CmdPackagePublishFlagSigningKeyPassword = "Password to the private key file used for publishing packages"
	CmdPackagePublishErr                    = "Failed to publish package: %s"

	CmdPackagePullShort               = "Pulls a Zarf package from a remote registry and save to the local file system"
	CmdPackagePullExample             = "	zarf package pull oci://my-registry.com/my-namespace/my-package:0.0.1-arm64"
	CmdPackagePullPublicKey           = "Path to public key file for validating signed packages"
	CmdPackagePullFlagOutputDirectory = "Specify the output directory for the pulled Zarf package"
	CmdPackagePullFlagPublicKey       = "Path to public key file for validating signed packages"
	CmdPackagePullErr                 = "Failed to pull package: %s"

	CmdPackageChoose    = "Choose or type the package file"
	CmdPackageChooseErr = "Package path selection canceled: %s"

	// zarf prepare
	CmdPrepareShort = "Tools to help prepare assets for packaging"

	CmdPreparePatchGitShort = "Converts all .git URLs to the specified Zarf HOST and with the Zarf URL pattern in a given FILE.  NOTE:\n" +
		"This should only be used for manifests that are not mutated by the Zarf Agent Mutating Webhook."
	CmdPreparePatchGitOverwritePrompt = "Overwrite the file %s with these changes?"
	CmdPreparePatchGitOverwriteErr    = "Confirm overwrite canceled: %s"
	CmdPreparePatchGitFileReadErr     = "Unable to read the file %s"
	CmdPreparePatchGitFileWriteErr    = "Unable to write the changes back to the file"

	CmdPrepareSha256sumShort   = "Generates a SHA256SUM for the given file"
	CmdPrepareSha256sumHashErr = "Unable to compute the SHA256SUM hash"

	CmdPrepareFindImagesShort = "Evaluates components in a zarf file to identify images specified in their helm charts and manifests"
	CmdPrepareFindImagesLong  = "Evaluates components in a zarf file to identify images specified in their helm charts and manifests.\n\n" +
		"Components that have repos that host helm charts can be processed by providing the --repo-chart-path."
	CmdPrepareFindImagesErr = "Unable to find images for the package definition %s"

	CmdPrepareGenerateConfigShort = "Generates a config file for Zarf"
	CmdPrepareGenerateConfigLong  = "Generates a Zarf config file for controlling how the Zarf CLI operates. Optionally accepts a filename to write the config to.\n\n" +
		"The extension will determine the format of the config file, e.g. env-1.yaml, env-2.json, env-3.toml etc.\n" +
		"Accepted extensions are json, toml, yaml.\n\n" +
		"NOTE: This file must not already exist. If no filename is provided, the config will be written to the current working directory as zarf-config.toml."
	CmdPrepareGenerateConfigErr = "Unable to write the config file %s, make sure the file doesn't already exist"

	CmdPrepareFlagSet           = "Specify package variables to set on the command line (KEY=value). Note, if using a config file, this will be set by [package.create.set]."
	CmdPrepareFlagRepoChartPath = `If git repos hold helm charts, often found with gitops tools, specify the chart path, e.g. "/" or "/chart"`
	CmdPrepareFlagGitAccount    = "User or organization name for the git account that the repos are created under."
	CmdPrepareFlagKubeVersion   = "Override the default helm template KubeVersion when performing a package chart template"

	// zarf tools
	CmdToolsShort = "Collection of additional tools to make airgap easier"

	CmdToolsArchiverShort           = "Compresses/Decompresses generic archives, including Zarf packages"
	CmdToolsArchiverCompressShort   = "Compresses a collection of sources based off of the destination file extension."
	CmdToolsArchiverCompressErr     = "Unable to perform compression: %s"
	CmdToolsArchiverDecompressShort = "Decompresses an archive or Zarf package based off of the source file extension."
	CmdToolsArchiverDecompressErr   = "Unable to perform decompression: %s"

	CmdToolsArchiverUnarchiveAllErr = "Unable to unarchive all nested tarballs"

	CmdToolsRegistryShort          = "Tools for working with container registries using go-containertools"
	CmdToolsRegistryCatalogExample = `
	# list the repos internal to Zarf
	$ zarf tools registry catalog

	# list the repos for reg.example.com
	$ zarf tools registry catalog reg.example.com
`
	CmdToolsRegistryListExample = `
	# list the tags for a repo internal to Zarf
	$ zarf tools registry ls 127.0.0.1:31999/stefanprodan/podinfo

	# list the tags for a repo hosted at reg.example.com
	$ zarf tools registry ls reg.example.com/stefanprodan/podinfo
`

	CmdToolsRegistryPushExample = `
	# push an image into an internal repo in Zarf
	$ zarf tools registry push image.tar 127.0.0.1:31999/stefanprodan/podinfo:6.4.0

	# push an image into an repo hosted at reg.example.com
	$ zarf tools registry push image.tar reg.example.com/stefanprodan/podinfo:6.4.0
`

	CmdToolsRegistryPullExample = `
	# pull an image from an internal repo in Zarf to a local tarball
	$ zarf tools registry pull 127.0.0.1:31999/stefanprodan/podinfo:6.4.0 image.tar

	# pull an image from a repo hosted at reg.example.com to a local tarball
	$ zarf tools registry pull reg.example.com/stefanprodan/podinfo:6.4.0 image.tar
`

	CmdToolsRegistryDeleteExample = `
# delete an image digest from an internal repo in Zarf
$ zarf tools registry delete 127.0.0.1:31999/stefanprodan/podinfo@sha256:57a654ace69ec02ba8973093b6a786faa15640575fbf0dbb603db55aca2ccec8

# delete an image digest from a repo hosted at reg.example.com
$ zarf tools registry delete reg.example.com/stefanprodan/podinfo@sha256:57a654ace69ec02ba8973093b6a786faa15640575fbf0dbb603db55aca2ccec8
`

	CmdToolsRegistryDigestExample = `
# return an image digest for an internal repo in Zarf
$ zarf tools registry digest 127.0.0.1:31999/stefanprodan/podinfo:6.4.0

# return an image digest from a repo hosted at reg.example.com
$ zarf tools registry digest reg.example.com/stefanprodan/podinfo:6.4.0
`

	CmdToolsRegistryPruneShort       = "Prunes images from the registry that are not currently being used by any Zarf packages."
	CmdToolsRegistryPruneFlagConfirm = "Confirm the image prune action to prevent accidental deletions"
	CmdToolsRegistryPruneImageList   = "The following image digests will be pruned from the registry:"
	CmdToolsRegistryPruneNoImages    = "There are no images to prune"

	CmdToolsRegistryInvalidPlatformErr = "Invalid platform '%s': %s"
	CmdToolsRegistryFlagVerbose        = "Enable debug logs"
	CmdToolsRegistryFlagInsecure       = "Allow image references to be fetched without TLS"
	CmdToolsRegistryFlagNonDist        = "Allow pushing non-distributable (foreign) layers"
	CmdToolsRegistryFlagPlatform       = "Specifies the platform in the form os/arch[/variant][:osversion] (e.g. linux/amd64)."

	CmdToolsGetGitPasswdShort       = "Deprecated: Returns the push user's password for the Git server"
	CmdToolsGetGitPasswdLong        = "Deprecated: Reads the password for a user with push access to the configured Git server in Zarf State. Note that this command has been replaced by 'zarf tools get-creds git' and will be removed in Zarf v1.0.0."
	CmdToolsGetGitPasswdDeprecation = "Deprecated: This command has been replaced by 'zarf tools get-creds git' and will be removed in Zarf v1.0.0."

	CmdToolsMonitorShort = "Launches a terminal UI to monitor the connected cluster using K9s."

	CmdToolsHelmShort = "Subset of the Helm CLI included with Zarf to help manage helm charts."
	CmdToolsHelmLong  = "Subset of the Helm CLI that includes the repo and dependency commands for managing helm charts destined for the air gap."

	CmdToolsClearCacheShort         = "Clears the configured git and image cache directory"
	CmdToolsClearCacheDir           = "Cache directory set to: %s"
	CmdToolsClearCacheErr           = "Unable to clear the cache directory %s"
	CmdToolsClearCacheSuccess       = "Successfully cleared the cache from %s"
	CmdToolsClearCacheFlagCachePath = "Specify the location of the Zarf artifact cache (images and git repositories)"

	CmdToolsCraneNotEnoughArgumentsErr   = "You do not have enough arguments specified for this command"
	CmdToolsCraneConnectedButBadStateErr = "Detected a K8s cluster but was unable to get Zarf state - continuing without state information: %s"

	CmdToolsDownloadInitShort               = "Downloads the init package for the current Zarf version into the specified directory"
	CmdToolsDownloadInitFlagOutputDirectory = "Specify a directory to place the init package in."
	CmdToolsDownloadInitErr                 = "Unable to download the init package: %s"

	CmdToolsGenPkiShort       = "Generates a Certificate Authority and PKI chain of trust for the given host"
	CmdToolsGenPkiSuccess     = "Successfully created a chain of trust for %s"
	CmdToolsGenPkiFlagAltName = "Specify Subject Alternative Names for the certificate"

	CmdToolsGenKeyShort                 = "Generates a cosign public/private keypair that can be used to sign packages"
	CmdToolsGenKeyPrompt                = "Private key password (empty for no password): "
	CmdToolsGenKeyPromptAgain           = "Private key password again (empty for no password): "
	CmdToolsGenKeyPromptExists          = "File %s already exists. Overwrite? "
	CmdToolsGenKeyErrUnableGetPassword  = "unable to get password for private key: %s"
	CmdToolsGenKeyErrPasswordsNotMatch  = "passwords do not match"
	CmdToolsGenKeyErrUnableToGenKeypair = "unable to generate key pair: %s"
	CmdToolsGenKeyErrNoConfirmOverwrite = "did not receive confirmation for overwriting key file(s)"
	CmdToolsGenKeySuccess               = "Generated key pair and written to %s and %s"

	CmdToolsSbomShort = "Generates a Software Bill of Materials (SBOM) for the given package"
	CmdToolsSbomErr   = "Unable to create SBOM (Syft) CLI"

	CmdToolsWaitForShort = "Waits for a given Kubernetes resource to be ready"
	CmdToolsWaitForLong  = "By default Zarf will wait for all Kubernetes resources to be ready before completion of a component during a deployment.\n" +
		"This command can be used to wait for a Kubernetes resources to exist and be ready that may be created by a Gitops tool or a Kubernetes operator.\n" +
		"You can also wait for arbitrary network endpoints using REST or TCP checks.\n\n"
	CmdToolsWaitForExample = `
	# Wait for Kubernetes resources:
	zarf tools wait-for pod my-pod-name ready -n default                  #  wait for pod my-pod-name in namespace default to be ready
	zarf tools wait-for p cool-pod-name ready -n cool                     #  wait for pod (using p alias) cool-pod-name in namespace cool to be ready
	zarf tools wait-for deployment podinfo available -n podinfo           #  wait for deployment podinfo in namespace podinfo to be available
	zarf tools wait-for pod app=podinfo ready -n podinfo                  #  wait for pod with label app=podinfo in namespace podinfo to be ready
	zarf tools wait-for svc zarf-docker-registry exists -n zarf           #  wait for service zarf-docker-registry in namespace zarf to exist
	zarf tools wait-for svc zarf-docker-registry -n zarf                  #  same as above, except exists is the default condition
	zarf tools wait-for crd addons.k3s.cattle.io                          #  wait for crd addons.k3s.cattle.io to exist
	zarf tools wait-for sts test-sts '{.status.availableReplicas}'=23     #  wait for statefulset test-sts to have 23 available replicas

	# Wait for network endpoints:
	zarf tools wait-for http localhost:8080 200                           #  wait for a 200 response from http://localhost:8080
	zarf tools wait-for tcp localhost:8080                                #  wait for a connection to be established on localhost:8080
	zarf tools wait-for https 1.1.1.1 200                                 #  wait for a 200 response from https://1.1.1.1
	zarf tools wait-for http google.com                                   #  wait for any 2xx response from http://google.com
	zarf tools wait-for http google.com success                           #  wait for any 2xx response from http://google.com
`
	CmdToolsWaitForFlagTimeout        = "Specify the timeout duration for the wait command."
	CmdToolsWaitForErrTimeoutString   = "Invalid timeout duration '%s'. Please use a valid duration string (e.g. 1s, 2m, 3h)."
	CmdToolsWaitForErrTimeout         = "Wait timed out."
	CmdToolsWaitForErrConditionString = "Invalid HTTP status code. Please use a valid HTTP status code (e.g. 200, 404, 500)."
	CmdToolsWaitForErrZarfPath        = "Could not locate the current Zarf binary path."
	CmdToolsWaitForFlagNamespace      = "Specify the namespace of the resources to wait for."

	CmdToolsKubectlDocs = "Kubectl command. See https://kubernetes.io/docs/reference/kubectl/overview/ for more information."

	CmdToolsGetCredsShort   = "Displays a table of credentials for deployed Zarf services. Pass a service key to get a single credential"
	CmdToolsGetCredsLong    = "Display a table of credentials for deployed Zarf services. Pass a service key to get a single credential. i.e. 'zarf tools get-creds registry'"
	CmdToolsGetCredsExample = `
	# Print all Zarf credentials:
	zarf tools get-creds

	# Get specific Zarf credentials:
	zarf tools get-creds registry
	zarf tools get-creds registry-readonly
	zarf tools get-creds git
	zarf tools get-creds git-readonly
	zarf tools get-creds artifact
	zarf tools get-creds logging
`

	CmdToolsUpdateCredsShort   = "Updates the credentials for deployed Zarf services. Pass a service key to update credentials for a single service"
	CmdToolsUpdateCredsLong    = "Updates the credentials for deployed Zarf services. Pass a service key to update credentials for a single service. i.e. 'zarf tools update-creds registry'"
	CmdToolsUpdateCredsExample = `
	# Autogenerate all Zarf credentials at once:
	zarf tools update-creds

	# Autogenerate specific Zarf service credentials:
	zarf tools update-creds registry
	zarf tools update-creds git
	zarf tools update-creds artifact
	zarf tools update-creds logging

	# Update all Zarf credentials w/external services at once:
	zarf tools update-creds \
		--registry-push-username={USERNAME} --registry-push-password={PASSWORD} \
		--git-push-username={USERNAME} --git-push-password={PASSWORD} \
		--artifact-push-username={USERNAME} --artifact-push-token={PASSWORD}

	# NOTE: Any credentials omitted from flags without a service key specified will be autogenerated - URLs will only change if specified.
	# Config options can also be set with the 'init' section of a Zarf config file.

	# Update specific Zarf credentials w/external services:
	zarf tools update-creds registry --registry-push-username={USERNAME} --registry-push-password={PASSWORD}
	zarf tools update-creds git --git-push-username={USERNAME} --git-push-password={PASSWORD}
	zarf tools update-creds artifact --artifact-push-username={USERNAME} --artifact-push-token={PASSWORD}

	# NOTE: Not specifying a pull username/password will keep the previous pull username/password.
`
	CmdToolsUpdateCredsConfirmFlag          = "Confirm updating credentials without prompting"
	CmdToolsUpdateCredsConfirmProvided      = "Confirm flag specified, continuing without prompting."
	CmdToolsUpdateCredsConfirmContinue      = "Continue with these changes?"
	CmdToolsUpdateCredsInvalidServiceErr    = "Invalid service key specified - valid keys are: %s, %s, and %s"
	CmdToolsUpdateCredsUnableCreateToken    = "Unable to create the new Gitea artifact token: %s"
	CmdToolsUpdateCredsUnableUpdateRegistry = "Unable to update Zarf registry: %s"
	CmdToolsUpdateCredsUnableUpdateGit      = "Unable to update Zarf git server: %s"

	// zarf version
	CmdVersionShort = "Shows the version of the running Zarf binary"
	CmdVersionLong  = "Displays the version of the Zarf release that the current binary was built from."

	// cmd viper setup
	CmdViperErrLoadingConfigFile = "failed to load config file: %s"
	CmdViperInfoUsingConfigFile  = "Using config file %s"
)

// Zarf Agent messages
// These are only seen in the Kubernetes logs.
const (
	AgentInfoWebhookAllowed = "Webhook [%s - %s] - Allowed: %t"
	AgentInfoShutdown       = "Shutdown gracefully..."
	AgentInfoPort           = "Server running in port: %s"

	AgentErrBadRequest             = "could not read request body: %s"
	AgentErrBindHandler            = "Unable to bind the webhook handler"
	AgentErrCouldNotDeserializeReq = "could not deserialize request: %s"
	AgentErrGetState               = "failed to load zarf state from file: %w"
	AgentErrHostnameMatch          = "failed to complete hostname matching: %w"
	AgentErrImageSwap              = "Unable to swap the host for (%s)"
	AgentErrInvalidMethod          = "invalid method only POST requests are allowed"
	AgentErrInvalidOp              = "invalid operation: %s"
	AgentErrInvalidType            = "only content type 'application/json' is supported"
	AgentErrMarshallJSONPatch      = "unable to marshall the json patch"
	AgentErrMarshalResponse        = "unable to marshal the response"
	AgentErrNilReq                 = "malformed admission review: request is nil"
	AgentErrShutdown               = "unable to properly shutdown the web server"
	AgentErrStart                  = "Failed to start the web server"
	AgentErrUnableTransform        = "unable to transform the provided request; see zarf http proxy logs for more details"
)

// src/internal/packager/create
const (
	PkgCreateErrDifferentialSameVersion = "unable to create a differential package with the same version as the package you are using as a reference; the package version must be incremented"
)

// src/internal/packager/validate.
const (
	PkgValidateTemplateDeprecation        = "Package template '%s' is using the deprecated syntax ###ZARF_PKG_VAR_%s###.  This will be removed in Zarf v1.0.0.  Please update to ###ZARF_PKG_TMPL_%s###."
	PkgValidateMustBeUppercase            = "variable name '%s' must be all uppercase and contain no special characters except _"
	PkgValidateErrAction                  = "invalid action: %w"
	PkgValidateErrActionVariables         = "component %s cannot contain setVariables outside of onDeploy in actions"
	PkgValidateErrActionCmdWait           = "action %s cannot be both a command and wait action"
	PkgValidateErrActionClusterNetwork    = "a single wait action must contain only one of cluster or network"
	PkgValidateErrChart                   = "invalid chart definition: %w"
	PkgValidateErrChartName               = "chart %s exceed the maximum length of %d characters"
	PkgValidateErrChartNameMissing        = "chart %s must include a name"
	PkgValidateErrChartNameNotUnique      = "chart name %q is not unique"
	PkgValidateErrChartNamespaceMissing   = "chart %s must include a namespace"
	PkgValidateErrChartURLOrPath          = "chart %s must only have a url or localPath"
	PkgValidateErrChartVersion            = "chart %s must include a chart version"
	PkgValidateErrComponentNameNotUnique  = "component name '%s' is not unique"
	PkgValidateErrComponent               = "invalid component: %w"
	PkgValidateErrComponentReqDefault     = "component %s cannot be both required and default"
	PkgValidateErrComponentReqGrouped     = "component %s cannot be both required and grouped"
	PkgValidateErrComponentYOLO           = "component %s incompatible with the online-only package flag (metadata.yolo): %w"
	PkgValidateErrConstant                = "invalid package constant: %w"
	PkgValidateErrImportPathInvalid       = "invalid file path '%s' provided directory must contain a valid zarf.yaml file"
	PkgValidateErrImportURLInvalid        = "invalid url '%s' provided"
	PkgValidateErrImportOptions           = "imported package %s must have either a url or a path"
	PkgValidateErrImportPathMissing       = "imported package %s must include a path"
	PkgValidateErrInitNoYOLO              = "sorry, you can't YOLO an init package"
	PkgValidateErrManifest                = "invalid manifest definition: %w"
	PkgValidateErrManifestFileOrKustomize = "manifest %s must have at least one file or kustomization"
	PkgValidateErrManifestNameLength      = "manifest %s exceed the maximum length of %d characters"
	PkgValidateErrManifestNameMissing     = "manifest %s must include a name"
	PkgValidateErrManifestNameNotUnique   = "manifest name %q is not unique"
	PkgValidateErrName                    = "invalid package name: %w"
	PkgValidateErrPkgConstantName         = "constant name '%s' must be all uppercase and contain no special characters except _"
	PkgValidateErrPkgConstantPattern      = "provided value for constant %q does not match pattern \"%s\""
	PkgValidateErrPkgName                 = "package name '%s' must be all lowercase and contain no special characters except -"
	PkgValidateErrVariable                = "invalid package variable: %w"
	PkgValidateErrYOLONoArch              = "cluster architecture not allowed"
	PkgValidateErrYOLONoDistro            = "cluster distros not allowed"
	PkgValidateErrYOLONoGit               = "git repos not allowed"
	PkgValidateErrYOLONoOCI               = "OCI images not allowed"
)

// Collection of reusable error messages.
var (
	ErrInitNotFound        = errors.New("this command requires a zarf-init package, but one was not found on the local system. Re-run the last command again without '--confirm' to download the package")
	ErrUnableToCheckArch   = errors.New("unable to get the configured cluster's architecture")
	ErrInterrupt           = errors.New("execution cancelled due to an interrupt")
	ErrUnableToGetPackages = errors.New("unable to load the Zarf Package data from the cluster")
)

// Collection of reusable warn messages.
var (
	WarnSGetDeprecation = "Using sget to download resources is being deprecated and will removed in the v1.0.0 release of Zarf. Please publish the packages as OCI artifacts instead."
)
