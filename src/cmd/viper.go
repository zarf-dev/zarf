// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"os"
	"strings"

	"github.com/defenseunicorns/zarf/src/cmd/tools"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/spf13/viper"
)

const (
	// Root config keys
	V_LOG_LEVEL    = "log_level"
	V_ARCHITECTURE = "architecture"
	V_NO_LOG_FILE  = "no_log_file"
	V_NO_PROGRESS  = "no_progress"
	V_ZARF_CACHE   = "zarf_cache"
	V_TMP_DIR      = "tmp_dir"
	V_INSECURE     = "insecure"

	// Init config keys
	V_INIT_COMPONENTS    = "init.components"
	V_INIT_STORAGE_CLASS = "init.storage_class"

	// Init Git config keys
	V_INIT_GIT_URL       = "init.git.url"
	V_INIT_GIT_PUSH_USER = "init.git.push_username"
	V_INIT_GIT_PUSH_PASS = "init.git.push_password"
	V_INIT_GIT_PULL_USER = "init.git.pull_username"
	V_INIT_GIT_PULL_PASS = "init.git.pull_password"

	// Init Registry config keys
	V_INIT_REGISTRY_URL       = "init.registry.url"
	V_INIT_REGISTRY_NODEPORT  = "init.registry.nodeport"
	V_INIT_REGISTRY_SECRET    = "init.registry.secret"
	V_INIT_REGISTRY_PUSH_USER = "init.registry.push_username"
	V_INIT_REGISTRY_PUSH_PASS = "init.registry.push_password"
	V_INIT_REGISTRY_PULL_USER = "init.registry.pull_username"
	V_INIT_REGISTRY_PULL_PASS = "init.registry.pull_password"

	// Init Package config keys
	V_INIT_ARTIFACT_URL        = "init.artifact.url"
	V_INIT_ARTIFACT_PUSH_USER  = "init.artifact.push_username"
	V_INIT_ARTIFACT_PUSH_TOKEN = "init.artifact.push_token"

	// Package config keys
	V_PKG_OCI_CONCURRENCY = "package.oci_concurrency"

	// Package create config keys
	V_PKG_CREATE_SET                  = "package.create.set"
	V_PKG_CREATE_OUTPUT               = "package.create.output"
	V_PKG_CREATE_SBOM                 = "package.create.sbom"
	V_PKG_CREATE_SBOM_OUTPUT          = "package.create.sbom_output"
	V_PKG_CREATE_SKIP_SBOM            = "package.create.skip_sbom"
	V_PKG_CREATE_MAX_PACKAGE_SIZE     = "package.create.max_package_size"
	V_PKG_CREATE_SIGNING_KEY          = "package.create.signing_key"
	V_PKG_CREATE_SIGNING_KEY_PASSWORD = "package.create.signing_key_password"
	V_PKG_CREATE_DIFFERENTIAL         = "package.create.differential"
	V_PKG_CREATE_REGISTRY_OVERRIDE    = "package.create.registry_override"

	// Package deploy config keys
	V_PKG_DEPLOY_SET        = "package.deploy.set"
	V_PKG_DEPLOY_COMPONENTS = "package.deploy.components"
	V_PKG_DEPLOY_SHASUM     = "package.deploy.shasum"
	V_PKG_DEPLOY_SGET       = "package.deploy.sget"
	V_PKG_DEPLOY_PUBLIC_KEY = "package.deploy.public_key"

	// Package publish config keys
	V_PKG_PUBLISH_SIGNING_KEY          = "package.publish.signing_key"
	V_PKG_PUBLISH_SIGNING_KEY_PASSWORD = "package.publish.signing_key_password"

	// Package pull config keys
	V_PKG_PULL_OUTPUT_DIR = "package.pull.output_directory"
	V_PKG_PULL_PUBLIC_KEY = "package.pull.public_key"

	// Bundle config keys
	V_BNDL_OCI_CONCURRENCY = "bundle.oci_concurrency"

	// Bundle create config keys
	V_BNDL_CREATE_OUTPUT               = "bundle.create.output"
	V_BNDL_CREATE_SIGNING_KEY          = "bundle.create.signing_key"
	V_BNDL_CREATE_SIGNING_KEY_PASSWORD = "bundle.create.signing_key_password"
	V_BNDL_CREATE_SET                  = "bundle.create.set"

	// Bundle deploy config keys
	V_BNDL_DEPLOY_PACKAGES = "bundle.deploy.packages"
	V_BNDL_DEPLOY_SET      = "bundle.deploy.set"

	// Bundle inspect config keys
	V_BNDL_INSPECT_KEY = "bundle.inspect.key"

	// Bundle remove config keys
	V_BNDL_REMOVE_PACKAGES = "bundle.remove.packages"

	// Bundle pull config keys
	V_BNDL_PULL_OUTPUT = "bundle.pull.output"
	V_BNDL_PULL_KEY    = "bundle.pull.key"
)

func initViper() {
	// Already initialized by some other command
	if v != nil {
		return
	}

	v = viper.New()

	// Skip for vendor-only commands
	if tools.CheckVendorOnlyFromArgs() {
		return
	}

	// Specify an alternate config file
	cfgFile := os.Getenv("ZARF_CONFIG")

	// Don't forget to read config either from cfgFile or from home directory!
	if cfgFile != "" {
		// Use config file from the flag.
		v.SetConfigFile(cfgFile)
	} else {
		// Search config paths in the current directory and $HOME/.zarf.
		v.AddConfigPath(".")
		v.AddConfigPath("$HOME/.zarf")
		v.SetConfigName("zarf-config")
	}

	// E.g. ZARF_LOG_LEVEL=debug
	v.SetEnvPrefix("zarf")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Optional, so ignore errors
	err := v.ReadInConfig()

	if err != nil {
		// Config file not found; ignore
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			message.WarnErrorf(err, lang.CmdViperErrLoadingConfigFile, err.Error())
		}
	} else {
		message.Notef(lang.CmdViperInfoUsingConfigFile, v.ConfigFileUsed())
	}
}
