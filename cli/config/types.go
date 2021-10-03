package config

type ZarfFile struct {
	Source     string `yaml:"source"`
	Shasum     string `yaml:"shasum"`
	Target     string `yaml:"target"`
	Executable bool   `yaml:"executable"`
}

type ZarfChart struct {
	Name    string `yaml:"name"`
	Url     string `yaml:"url"`
	Version string `yaml:"version"`
}

type ZarfComponent struct {
	Name        string      `yaml:"name"`
	Description string      `yaml:"description"`
	Default     bool        `yaml:"default"`
	Required    bool        `yaml:"required"`
	Manifests   string      `yaml:"manifests"`
	Images      []string    `yaml:"images"`
	Files       []ZarfFile  `yaml:"files"`
	Charts      []ZarfChart `yaml:"charts"`
}

type ZarfMetatdata struct {
	Name         string `yaml:"name"`
	Description  string `yaml:"description"`
	Version      string `yaml:"version"`
	Uncompressed bool   `yaml:"uncompressed"`
}

type ZarfContainerTarget struct {
	Namespace string `yaml:"namespace"`
	Selector  string `yaml:"selector"`
	Container string `yaml:"container"`
	Path      string `yaml:"path"`
}

type ZarfData struct {
	Source string              `yaml:"source"`
	Target ZarfContainerTarget `yaml:"target"`
}

type ZarfBuildData struct {
	Terminal  string `yaml:"terminal"`
	User      string `yaml:"user"`
	Timestamp string `yaml:"timestamp"`
}

type ZarfUtilityCluster struct {
	Images []string `yaml:"images"`
	Repos  []string `yaml:"repos"`
}

type ZarfConfig struct {
	Kind           string             `yaml:"kind"`
	Metadata       ZarfMetatdata      `yaml:"metadata"`
	Package        ZarfBuildData      `yaml:"package"`
	Data           []ZarfData         `yaml:"data"`
	Components     []ZarfComponent    `yaml:"components"`
	UtilityCluster ZarfUtilityCluster `yaml:"utilityCluster"`
}
