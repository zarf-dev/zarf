// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package requirements

type requirementsFile struct {
	Agent   *agentRequirements   `json:"agent,omitempty" yaml:"agent,omitempty"`
	Cluster *clusterRequirements `json:"cluster,omitempty" yaml:"cluster,omitempty"`
}

type agentRequirements struct {
	Tools []toolRequirement `json:"tools,omitempty" yaml:"tools,omitempty"`
	Env   []envRequirement  `json:"env,omitempty" yaml:"env,omitempty"`
}

type toolRequirement struct {
	Name    string `json:"name" yaml:"name"`
	Version string `json:"version,omitempty" yaml:"version,omitempty"` // semver constraint, e.g. ">= 4.40.5"
	// Optional enhancements (future-proofing):
	VersionCommand string `json:"versionCommand,omitempty" yaml:"versionCommand,omitempty"`
	VersionRegex   string `json:"versionRegex,omitempty" yaml:"versionRegex,omitempty"`
	Optional       bool   `json:"optional,omitempty" yaml:"optional,omitempty"`
	Reason         string `json:"reason,omitempty" yaml:"reason,omitempty"`
}

type envRequirement struct {
	Name     string `json:"name" yaml:"name"`
	Required bool   `json:"required" yaml:"required"`
	Reason   string `json:"reason,omitempty" yaml:"reason,omitempty"`
}

type clusterRequirements struct {
	Packages  []packageRequirement  `json:"packages,omitempty" yaml:"packages,omitempty"`
	CRDs      []crdRequirement      `json:"crds,omitempty" yaml:"crds,omitempty"`
	Resources []k8sResourceSelector `json:"resources,omitempty" yaml:"resources,omitempty"`
}

type crdRequirement struct {
	Name     string `json:"name" yaml:"name"` // e.g. "certificates.cert-manager.io"
	Version  string `json:"version,omitempty" yaml:"version,omitempty"`
	Optional bool   `json:"optional,omitempty" yaml:"optional,omitempty"`
	Reason   string `json:"reason,omitempty" yaml:"reason,omitempty"`
}

type k8sResourceSelector struct {
	APIVersion string `json:"apiVersion" yaml:"apiVersion"`
	Kind       string `json:"kind" yaml:"kind"`
	Namespace  string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Name       string `json:"name" yaml:"name"`
	Optional   bool   `json:"optional,omitempty" yaml:"optional,omitempty"`
	Reason     string `json:"reason,omitempty" yaml:"reason,omitempty"`
}

type packageRequirement struct {
	Name      string `json:"name" yaml:"name"`                           // package metadata.name
	Version   string `json:"version,omitempty" yaml:"version,omitempty"` // semver constraint, supports !=, ranges
	Optional  bool   `json:"optional,omitempty" yaml:"optional,omitempty"`
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"` // optional override; default zarf ns
	Reason    string `json:"reason,omitempty" yaml:"reason,omitempty"`
}
