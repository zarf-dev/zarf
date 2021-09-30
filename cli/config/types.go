package config

type ZarfFile struct {
	Source     string
	Shasum     string
	Target     string
	Executable bool
}

type ZarfChart struct {
	Name    string
	Url     string
	Version string
}

type ZarfComponent struct {
	Name        string
	Description string
	Default     bool
	Required    bool
	Manifests   string
	Images      []string
	Files       []ZarfFile
	Charts      []ZarfChart
}

type ZarfMetatdata struct {
	Name         string
	Description  string
	Version      string
	Uncompressed bool
}

type ZarfContainerTarget struct {
	Namespace string
	Selector  string
	Container string
	Path      string
}

type ZarfData struct {
	Source string
	Target ZarfContainerTarget
}

type ZarfConfig struct {
	Kind     string
	Metadata ZarfMetatdata
	Package  struct {
		Terminal  string
		User      string
		Timestamp string
	}
	Components     []ZarfComponent
	Data           []ZarfData
	UtilityCluster struct {
		Images []string
		Repos  []string
	}
}
