package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/defenseunicorns/zarf/src/types"

	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/utils"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

const (
	GithubProject = "defenseunicorns/zarf"
	IPV4Localhost = "127.0.0.1"

	PackagePrefix = "zarf-package"

	// ZarfMaxChartNameLength limits helm chart name size to account for K8s/helm limits and zarf prefix
	ZarfMaxChartNameLength  = 40
	ZarfGitPushUser         = "zarf-git-user"
	ZarfGitReadUser         = "zarf-git-read-user"
	ZarfRegistryPushUser    = "zarf-push"
	ZarfRegistryPullUser    = "zarf-pull"
	ZarfRegistry            = IPV4Localhost + ":45001"
	ZarfImagePullSecretName = "private-registry"
	ZarfGitServerSecretName = "private-git-server"

	ZarfAgentHost = "agent-hook.zarf.svc"

	ZarfConnectLabelName             = "zarf.dev/connect-name"
	ZarfConnectAnnotationDescription = "zarf.dev/connect-description"
	ZarfConnectAnnotationUrl         = "zarf.dev/connect-url"

	ZarfManagedByLabel        = "app.kubernetes.io/managed-by"
	ZarfCleanupScriptsPath    = "/opt/zarf"
	ZarfDefaultImageCachePath = ".zarf-image-cache"

	ZarfYAML = "zarf.yaml"
)

var (
	// CLIVersion track the version of the CLI
	CLIVersion = "unset"

	// CreeateOptions tracks the user-defined options used to create the package
	CreateOptions types.ZarfCreateOptions

	// DeployOptions tracks user-defined values for the active deployment
	DeployOptions types.ZarfDeployOptions

	CliArch string

	ZarfSeedPort string

	// Private vars
	active types.ZarfPackage
	state  types.ZarfState

	SGetPublicKey string
)

func IsZarfInitConfig() bool {
	message.Debug("config.IsZarfInitConfig")
	return strings.ToLower(active.Kind) == "zarfinitconfig"
}

func GetArch() string {
	// If CLI-orverriden then reflect that
	if CliArch != "" {
		return CliArch
	}

	if active.Metadata.Architecture != "" {
		return active.Metadata.Architecture
	}

	if active.Build.Architecture != "" {
		return active.Build.Architecture
	}

	return runtime.GOARCH
}

func GetCraneOptions() []crane.Option {
	var options []crane.Option

	// Handle insecure registry option
	if CreateOptions.Insecure {
		options = append(options, crane.Insecure)
	}

	// Add the image platform info
	options = append(options,
		crane.WithPlatform(&v1.Platform{
			OS:           "linux",
			Architecture: GetArch(),
		}),
	)

	return options
}

func GetCraneAuthOption(username string, secret string) crane.Option {
	return crane.WithAuth(
		authn.FromConfig(authn.AuthConfig{
			Username: username,
			Password: secret,
		}))
}

func GetSeedRegistry() string {
	return fmt.Sprintf("%s:%s", IPV4Localhost, ZarfSeedPort)
}

// GetSeedImage returns a list of image strings specified in the package, but only for init packages
func GetSeedImage() string {
	message.Debugf("config.GetSeedImage()")
	// Only allow seed images for init config
	if IsZarfInitConfig() {
		return active.Seed
	} else {
		return ""
	}
}

func GetPackageName() string {
	metadata := GetMetaData()
	prefix := PackagePrefix
	suffix := "tar.zst"

	if IsZarfInitConfig() {
		return fmt.Sprintf("zarf-init-%s.tar.zst", GetArch())
	}

	if metadata.Uncompressed {
		suffix = "tar"
	}
	return fmt.Sprintf("%s-%s-%s.%s", prefix, metadata.Name, GetArch(), suffix)
}

func GetMetaData() types.ZarfMetadata {
	return active.Metadata
}

func GetComponents() []types.ZarfComponent {
	return active.Components
}

func SetComponents(components []types.ZarfComponent) {
	active.Components = components
}

func GetBuildData() types.ZarfBuildData {
	return active.Build
}

func GetValidPackageExtensions() [3]string {
	return [...]string{".tar.zst", ".tar", ".zip"}
}

func InitState(tmpState types.ZarfState) {
	message.Debugf("config.InitState(%v)", tmpState)
	state = tmpState
	initSecrets()
}

func GetState() types.ZarfState {
	return state
}

func GetRegistry() string {
	return fmt.Sprintf("%s:%s", IPV4Localhost, state.NodePort)
}

// LoadConfig loads the config from the given path and removes
// components not matching the current OS if filterByOS is set.
func LoadConfig(path string, filterByOS bool) error {
	if err := utils.ReadYaml(path, &active); err != nil {
		return err
	}

	// Filter each component to only compatible platforms
	filteredComponents := []types.ZarfComponent{}
	for _, component := range active.Components {
		if isCompatibleComponent(component, filterByOS) {
			filteredComponents = append(filteredComponents, component)
		}
	}
	// Update the active package with the filtered components
	active.Components = filteredComponents

	return nil
}

func GetActiveConfig() types.ZarfPackage {
	return active
}

func BuildConfig(path string) error {
	message.Debugf("config.BuildConfig(%v)", path)
	now := time.Now()
	// Just use $USER env variable to avoid CGO issue
	// https://groups.google.com/g/golang-dev/c/ZFDDX3ZiJ84
	currentUser := os.Getenv("USER")
	hostname, hostErr := os.Hostname()

	// Need to ensure the arch is updated if injected
	arch := GetArch()

	// Normalize these for the package confirmation
	active.Metadata.Architecture = arch
	active.Build.Architecture = arch

	// Record the time of package creation
	active.Build.Timestamp = now.Format(time.RFC1123Z)

	// Record the Zarf Version the CLI was built with
	active.Build.Version = CLIVersion

	if hostErr == nil {
		// Record the hostname of the package creation terminal
		active.Build.Terminal = hostname
	}

	// Record the name of the user creating the package
	active.Build.User = currentUser

	return utils.WriteYaml(path, active, 0400)
}

func SetImageCachePath(cachePath string) {
	CreateOptions.ImageCachePath = cachePath
}

func GetImageCachePath() string {
	homePath, _ := os.UserHomeDir()

	if CreateOptions.ImageCachePath == "" {
		return filepath.Join(homePath, ZarfDefaultImageCachePath)
	}

	return strings.Replace(CreateOptions.ImageCachePath, "~", homePath, 1)
}

func isCompatibleComponent(component types.ZarfComponent, filterByOS bool) bool {
	// Ignore only filters that are empty
	var validArch, validOS bool

	targetArch := GetArch()

	// Test for valid architecture
	if component.Only.ClusterArch == "" || component.Only.ClusterArch == targetArch {
		validArch = true
	}

	// Test for a valid OS
	if !filterByOS || component.Only.LocalOS == "" || component.Only.LocalOS == runtime.GOOS {
		validOS = true
	}

	return validArch && validOS
}
