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

	// ZarfMaxChartNameLength limits helm chart name size to account for K8s/helm limits and zarf prefix
	ZarfMaxChartNameLength = 40

	ZarfAgentHost = "agent-hook.zarf.svc"

	ZarfConnectLabelName             = "zarf.dev/connect-name"
	ZarfConnectAnnotationDescription = "zarf.dev/connect-description"
	ZarfConnectAnnotationURL         = "zarf.dev/connect-url"

	ZarfManagedByLabel     = "app.kubernetes.io/managed-by"
	ZarfCleanupScriptsPath = "/opt/zarf"

	ZarfPackagePrefix = "zarf-package-"

	ZarfDeployStage = "Deploy"
	ZarfCreateStage = "Create"
	ZarfMirrorStage = "Mirror"
)

// Zarf Constants for In-Cluster Services.
const (
	ZarfArtifactTokenName = "zarf-artifact-registry-token"

	ZarfImagePullSecretName = "private-registry"
	ZarfGitServerSecretName = "private-git-server"

	ZarfLoggingUser = "zarf-admin"

	UnsetCLIVersion = "unset-development-only"
)

// Zarf Global Configuration Variables.
var (
	// CLIVersion track the version of the CLI
	CLIVersion = UnsetCLIVersion

	// ActionsUseSystemZarf sets whether to use Zarf from the system path if Zarf is being used as a library
	ActionsUseSystemZarf = false

	// ActionsCommandZarfPrefix sets a sub command prefix that Zarf commands are under in the current binary if Zarf is being used as a library (and use system Zarf is not specified)
	ActionsCommandZarfPrefix = ""

	// CommonOptions tracks user-defined values that apply across commands
	CommonOptions types.ZarfCommonOptions

	// CLIArch is the computer architecture of the device executing the CLI commands
	CLIArch string

	// ZarfSeedPort is the NodePort Zarf uses for the 'seed registry'
	ZarfSeedPort string

	// SkipLogFile is a flag to skip logging to a file
	SkipLogFile bool

	// NoColor is a flag to disable colors in output
	NoColor bool

	CosignPublicKey string
	ZarfSchema      embed.FS

	// Timestamp of when the CLI was started
	operationStartTime  = time.Now().Unix()
	dataInjectionMarker = ".zarf-injection-%d"

	ZarfDefaultCachePath = filepath.Join("~", ".zarf-cache")

	// Default Time Vars
	ZarfDefaultTimeout = 15 * time.Minute
	ZarfDefaultRetries = 3
)

// GetArch returns the arch based on a priority list with options for overriding.
func GetArch(archs ...string) string {
	// List of architecture overrides.
	priority := append([]string{CLIArch}, archs...)

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
func GetCraneOptions(insecure bool, archs ...string) []crane.Option {
	var options []crane.Option

	// Handle insecure registry option
	if insecure {
		roundTripper := http.DefaultTransport.(*http.Transport).Clone()
		roundTripper.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
		options = append(options, crane.Insecure, crane.WithTransport(roundTripper))
	}

	if archs != nil {
		options = append(options, crane.WithPlatform(&v1.Platform{OS: "linux", Architecture: GetArch(archs...)}))
	}

	options = append(options,
		crane.WithUserAgent("zarf"),
		crane.WithNoClobber(true),
		// TODO: (@WSTARR) this is set to limit pushes to registry pods and reduce the likelihood that crane will get stuck.
		// We should investigate this further in the future to dig into more of what is happening (see https://github.com/defenseunicorns/zarf/issues/1568)
		crane.WithJobs(1),
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

// GetAbsCachePath gets the absolute cache path for images and git repos.
func GetAbsCachePath() string {
	return GetAbsHomePath(CommonOptions.CachePath)
}

// GetAbsHomePath replaces ~ with the absolute path to a user's home dir
func GetAbsHomePath(path string) string {
	homePath, _ := os.UserHomeDir()

	if strings.HasPrefix(path, "~") {
		return strings.Replace(path, "~", homePath, 1)
	}
	return path
}
