package types

// ZarfPackage the top-level structure of a Zarf config file
type ZarfPackage struct {
	Kind       string          `yaml:"kind" jsonschema:"description=The kind of Zarf package,pattern=^ZarfInitConfig|ZarfPackageConfig$,default=ZarfPackageConfig"`
	Metadata   ZarfMetadata    `yaml:"metadata,omitempty" jsonschema:"description=Package metadata"`
	Build      ZarfBuildData   `yaml:"build,omitempty" jsonschema:"description=Zarf-generated package build data"`
	Components []ZarfComponent `yaml:"components" jsonschema:"description=List of components to deploy in this package"`
	Seed       string          `yaml:"seed,omitempty" jsonschema:"description=Special image only used for ZarfInitConfig packages when used with the Zarf Injector"`
}

// ZarfMetadata lists information about the current ZarfPackage
type ZarfMetadata struct {
	Name         string `yaml:"name" jsonschema:"description=Name to identify this Zarf package,pattern=^[a-z0-9\\-]+$"`
	Description  string `yaml:"description,omitempty" jsonschema:"description=Additional information about this package"`
	Version      string `yaml:"version,omitempty" jsonschema:"description=Generic string to track the package version by a package author"`
	Url          string `yaml:"url,omitempty" jsonschema:"description=Link to package information when online"`
	Image        string `yaml:"image,omitempty" jsonschema:"description=An image URL to embed in this package for future Zarf UI listing"`
	Uncompressed bool   `yaml:"uncompressed,omitempty" jsonschema:"description=Disable compression of this package"`
	Architecture string `yaml:"architecture,omitempty" jsonschema:"description=The target cluster architecture of this package"`
}

// ZarfBuildData is written during the packager.Create() operation to track details of the created package
type ZarfBuildData struct {
	Terminal     string `yaml:"terminal"`
	User         string `yaml:"user"`
	Architecture string `yaml:"architecture"`
	Timestamp    string `yaml:"timestamp"`
	Version      string `yaml:"version"`
}
