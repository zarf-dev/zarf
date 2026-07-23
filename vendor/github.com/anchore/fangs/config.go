package fangs

import (
	"fmt"
	"os"

	"github.com/anchore/go-logger"
	"github.com/anchore/go-logger/adapter/discard"
)

type Config struct {
	// Logger should be provided for Fangs to log output
	Logger logger.Logger `yaml:"-" json:"-" mapstructure:"-"`

	// AppName is used to specify the name of files and environment variables to look for
	AppName string `yaml:"-" json:"-" mapstructure:"-"`

	// TagName is the struct tag to use for configuration structure field names (defaults to mapstructure)
	TagName string `yaml:"-" json:"-" mapstructure:"-"`

	// MultiFile allows for multiple configuration files, including hierarchical inheritance of files found in search locations when no files directly specified
	MultiFile bool `yaml:"-" json:"-" mapstructure:"-"`

	// Files is where configuration files are specified
	Files []string `yaml:"-" json:"-" mapstructure:"-"`

	// Finders are used to search for configuration when no files explicitly specified
	Finders []Finder `yaml:"-" json:"-" mapstructure:"-"`

	// ProfileKey is the top-level configuration key to define profiles
	ProfileKey string `yaml:"-" json:"-" mapstructure:"-"`

	// Profiles specific profiles to load
	Profiles []string `yaml:"-" json:"-" mapstructure:"-"`
}

var _ FlagAdder = (*Config)(nil)

// NewConfig creates a new Config object with defaults
func NewConfig(appName string) Config {
	return Config{
		Logger:     discard.New(),
		AppName:    appName,
		TagName:    "mapstructure",
		MultiFile:  true,
		ProfileKey: "profiles",
		// search for configs in specific order
		Finders: []Finder{
			// 2. look for ./.<appname>.<ext>
			FindInCwd,
			// 3. look for ./.<appname>/config.<ext>
			FindInAppNameSubdir,
			// 4. look for ~/.<appname>.<ext>
			FindInHomeDir,
			// 5. look for <appname>/config.<ext> in xdg locations
			FindInXDG,
		},
	}
}

// WithConfigEnvVar looks for the environment variable: <APP_NAME>_CONFIG as a way to specify a config file
// This will be overridden by a command-line flag
func (c Config) WithConfigEnvVar() Config {
	envConfig := os.Getenv(envVar(c.AppName, "CONFIG"))
	if envConfig != "" {
		c.Files = Flatten(envConfig)
	}
	return c
}

func (c *Config) AddFlags(flags FlagSet) {
	if c.MultiFile {
		flags.StringArrayVarP(&c.Files, "config", "c", fmt.Sprintf("%s configuration file(s) to use", c.AppName))
	} else {
		if len(c.Files) == 0 {
			// need a location to store a string reference
			c.Files = []string{""}
		}
		flags.StringVarP(&c.Files[0], "config", "c", fmt.Sprintf("%s configuration file", c.AppName))
	}
	if c.ProfileKey != "" {
		flags.StringArrayVarP(&c.Profiles, "profile", "", "configuration profiles to use")
	}
}
