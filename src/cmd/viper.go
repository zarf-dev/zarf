package cmd

import (
	"os"

	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/spf13/viper"
)

const (
	V_LOG_LEVEL    = "log_level"
	V_ARCHITECTURE = "architecture"
	V_NO_LOG_FILE  = "no_log_file"
	V_NO_PROGRESS  = "no_progress"
	V_TMP_DIR      = "tmp_dir"

	V_INIT_COMPONENTS    = "init.components"
	V_INIT_STORAGE_CLASS = "init.storage_class"
	V_INIT_SECRET        = "init.secret"
	V_INIT_NODEPORT      = "init.nodeport"

	V_PKG_CREATE_SET            = "package.create.set"
	V_PKG_CREATE_ZARF_CACHE     = "package.create.zarf_cache"
	V_PKG_CREATE_OUTPUT_DIRTORY = "package.create.output_directory"
	V_PKG_CREATE_SKIP_SBOM      = "package.create.skip_sbom"
	V_PKG_CREATE_INSECURE       = "package.create.insecure"

	V_PKG_DEPLOY_SET        = "package.deploy.set"
	V_PKG_DEPLOY_COMPONENTS = "package.deploy.components"
	V_PKG_DEPLOY_INSECURE   = "package.deploy.insecure"
	V_PKG_DEPLOY_SHASUM     = "package.deploy.shasum"
	V_PKG_DEPLOY_SGET       = "package.deploy.sget"
)

func initViper() {
	// Already initializedby some other command
	if v != nil {
		return
	}

	v = viper.New()
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

	v.SetEnvPrefix("zarf")
	v.AutomaticEnv()

	// E.g. ZARF_LOG_LEVEL=debug
	v.SetEnvPrefix("zarf")
	v.AutomaticEnv()

	// Optional, so ignore errors
	err := v.ReadInConfig()

	if err != nil {
		// Config file not found; ignore
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			message.Error(err, "Failed to read config file")
		}
	} else {
		message.Notef("Using config file %s", v.ConfigFileUsed())
	}
}
