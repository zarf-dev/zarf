// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package config stores the global configuration and constants.
package config

import (
	"crypto/tls"
	"embed"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/defenseunicorns/zarf/src/types"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// Zarf Global Configuration Constants.
const (
	GithubProject = "defenseunicorns/zarf"
	IPV4Localhost = "127.0.0.1"

	// ZarfMaxChartNameLength limits helm chart name size to account for K8s/helm limits and zarf prefix
	ZarfMaxChartNameLength   = 40
	ZarfGitPushUser          = "zarf-git-user"
	ZarfGitReadUser          = "zarf-git-read-user"
	ZarfRegistryPushUser     = "zarf-push"
	ZarfRegistryPullUser     = "zarf-pull"
	ZarfImagePullSecretName  = "private-registry"
	ZarfGitServerSecretName  = "private-git-server"
	ZarfGeneratedPasswordLen = 24
	ZarfGeneratedSecretLen   = 48

	ZarfAgentHost = "agent-hook.zarf.svc"

	ZarfConnectLabelName             = "zarf.dev/connect-name"
	ZarfConnectAnnotationDescription = "zarf.dev/connect-description"
	ZarfConnectAnnotationURL         = "zarf.dev/connect-url"

	ZarfManagedByLabel     = "app.kubernetes.io/managed-by"
	ZarfCleanupScriptsPath = "/opt/zarf"

	ZarfImageCacheDir = "images"

	ZarfYAML          = "zarf.yaml"
	ZarfSBOMDir       = "zarf-sbom"
	ZarfPackagePrefix = "zarf-package-"

	ZarfInClusterContainerRegistryNodePort = 31999

	ZarfInClusterGitServiceURL = "http://zarf-gitea-http.zarf.svc.cluster.local:3000"

	ZarfSeedImage = "registry"
	ZarfSeedTag   = "2.8.1"
)

// Zarf Global Configuration Variables.
var (
	// CLIVersion track the version of the CLI
	CLIVersion = "unset"

	// CommonOptions tracks user-defined values that apply across commands.
	CommonOptions types.ZarfCommonOptions

	// CliArch is the computer architecture of the device executing the CLI commands
	CliArch string

	// ZarfSeedPort is the NodePort Zarf uses for the 'seed registry'
	ZarfSeedPort string

	// Dirty Solution to getting the real time deployedComponents components.
	deployedComponents []types.DeployedComponent

	// SkipLogFile is a flag to skip logging to a file
	SkipLogFile bool

	SGetPublicKey string
	UIAssets      embed.FS

	// Timestamp of when the CLI was started
	operationStartTime  = time.Now().Unix()
	dataInjectionMarker = ".zarf-injection-%d"

	ZarfDefaultCachePath = filepath.Join("~", ".zarf-cache")
)

// GetArch returns the arch based on a priority list with options for overriding.
func GetArch(archs ...string) string {
	// List of architecture overrides.
	priority := append([]string{CliArch}, archs...)

	// Find the first architecture that is specified.
	for _, arch := range priority {
		if arch != "" {
			return arch
		}
	}

	return runtime.GOARCH
}

// GetStartTime returns the timestamp of when the CLI was started.
func GetStartTime() int64 {
	return operationStartTime
}

// GetDataInjectionMarker returns the data injection marker based on the current CLI start time.
func GetDataInjectionMarker() string {
	return fmt.Sprintf(dataInjectionMarker, operationStartTime)
}

// GetCraneOptions returns a crane option object with the correct options & platform.
func GetCraneOptions(insecure bool) []crane.Option {
	var options []crane.Option

	// Handle insecure registry option
	if insecure {
		roundTripper := http.DefaultTransport.(*http.Transport).Clone()
		roundTripper.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
		options = append(options, crane.Insecure, crane.WithTransport(roundTripper))
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

// GetCraneAuthOption returns a crane auth option with the provided credentials.
func GetCraneAuthOption(username string, secret string) crane.Option {
	return crane.WithAuth(
		authn.FromConfig(authn.AuthConfig{
			Username: username,
			Password: secret,
		}))
}

// GetDeployingComponents returns the list of deploying components.
// TODO: (@jeff-mccoy) this should be moved out of config.
func GetDeployingComponents() []types.DeployedComponent {
	return deployedComponents
}

// SetDeployingComponents sets the list of deploying components.
// TODO: (@jeff-mccoy) this should be moved out of config.
func SetDeployingComponents(components []types.DeployedComponent) {
	deployedComponents = components
}

// ClearDeployingComponents clears the list of deploying components.
// TODO: (@jeff-mccoy) this should be moved out of config.
func ClearDeployingComponents() {
	deployedComponents = []types.DeployedComponent{}
}

// GetValidPackageExtensions returns the valid package extensions.
func GetValidPackageExtensions() [3]string {
	return [...]string{".tar.zst", ".tar", ".zip"}
}

// GetAbsCachePath gets the absolute cache path for images and git repos.
func GetAbsCachePath() string {
	homePath, _ := os.UserHomeDir()

	if strings.HasPrefix(CommonOptions.CachePath, "~") {
		return strings.Replace(CommonOptions.CachePath, "~", homePath, 1)
	}
	return CommonOptions.CachePath
}
