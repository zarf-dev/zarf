package types

// ZarfPackage the top-level structure of a Zarf config file
type ZarfPackage struct {
	Kind       string          `yaml:"kind,omitempty"`
	Metadata   ZarfMetadata    `yaml:"metadata,omitempty"`
	Build      ZarfBuildData   `yaml:"build,omitempty"`
	Components []ZarfComponent `yaml:"components,omitempty"`
	Seed       string          `yaml:"seed,omitempty"`
}

// ZarfMetadata lists information about the current ZarfPackage
type ZarfMetadata struct {
	Name         string `yaml:"name,omitempty"`
	Description  string `yaml:"description,omitempty"`
	Version      string `yaml:"version,omitempty"`
	Url          string `yaml:"url,omitempty"`
	Image        string `yaml:"image,omitempty"`
	Uncompressed bool   `yaml:"uncompressed,omitempty"`
	Architecture string `yaml:"architecture,omitempty"`
}

// ZarfBuildData is written during the packager.Create() operation to track details of the created package
type ZarfBuildData struct {
	Terminal     string `yaml:"terminal"`
	User         string `yaml:"user"`
	Architecture string `yaml:"architecture"`
	Timestamp    string `yaml:"timestamp"`
	Version      string `yaml:"version"`
}
