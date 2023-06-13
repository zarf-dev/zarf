// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package extensions contains the types for all official extensions.
package extensions

// RetrieveImages defines a file to deploy.
type RetrieveImages struct {
	FromGitChart  []FromGitChart `json:"fromGitChart,omitempty" jsonschema:"description=Charts in git repositories to retrieve images from"`
	FromHelmChart []string       `json:"fromHelmChart,omitempty" jsonschema:"description=List of chart names declared in this component to retrieve images from"`
	FromManifest  []string       `json:"fromManifest,omitempty" jsonschema:"description=List of manifests declared in this component to retrieve images from"`
}

type FromGitChart struct {
	Url    string `json:"url" jsonschema:"description=The URL of the chart git repository"`
	Path   string `json:"path" jsonschema:"description=The path to the chart in the git repository"`
	Tag    string `json:"tag,omitempty" jsonschema:"description=The tag of the repository to pull"`
	Branch string `json:"branch,omitempty" jsonschema:"description=The branch of the repository to pull"`
}
