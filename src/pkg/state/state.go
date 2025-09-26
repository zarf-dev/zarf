// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package state manages references to a logical zarf deployment in k8s.
package state

import (
	"context"
	"fmt"
	"slices"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/pki"
)

// Declares secrets and metadata keys and values.
// TODO(mkcp): Remove Zarf prefix, that's the project name.
// TODO(mkcp): Provide semantic doccomments for how these are used.
const (
	ZarfManagedByLabel   = "app.kubernetes.io/managed-by"
	ZarfNamespaceName    = "zarf"
	ZarfStateSecretName  = "zarf-state"
	ZarfStateDataKey     = "state"
	ZarfPackageInfoLabel = "package-deploy-info"
)

// Credential keys
// TODO(mkcp): Provide semantic doccomments for how these are used.
const (
	RegistryKey     = "registry"
	RegistryReadKey = "registry-readonly"
	GitKey          = "git"
	GitReadKey      = "git-readonly"
	ArtifactKey     = "artifact"
	AgentKey        = "agent"
)

// ComponentStatus defines the deployment status of a Zarf component within a package.
type ComponentStatus string

// All the different status options for a Zarf Component
const (
	ComponentStatusSucceeded ComponentStatus = "Succeeded"
	ComponentStatusFailed    ComponentStatus = "Failed"
	ComponentStatusDeploying ComponentStatus = "Deploying"
	ComponentStatusRemoving  ComponentStatus = "Removing"
)

// IPFamily defines the different possible IPfamilies that can be used in Kubernetes clusters
type IPFamily string

// The possible IP stacks in a Kubernetes Cluster
const (
	IPFamilyIPv4      IPFamily = "ipv4"
	IPFamilyIPv6      IPFamily = "ipv6"
	IPFamilyDualStack IPFamily = "dual"
)

// All status options for a Zarf component chart
const (
	ChartStatusSucceeded ChartStatus = "Succeeded"
	ChartStatusFailed    ChartStatus = "Failed"
)

// Values during setup of the initial zarf state
const (
	ZarfGeneratedPasswordLen               = 24
	ZarfGeneratedSecretLen                 = 48
	ZarfInClusterContainerRegistryNodePort = 31999
	ZarfInjectorHostPort                   = 5001
	ZarfRegistryHostPort                   = 5000
	ZarfRegistryPushUser                   = "zarf-push"
	ZarfRegistryPullUser                   = "zarf-pull"

	ZarfGitPushUser = "zarf-git-user"
	ZarfGitReadUser = "zarf-git-read-user"
	ZarfAgentHost   = "agent-hook.zarf.svc"

	ZarfInClusterGitServiceURL      = "http://zarf-gitea-http.zarf.svc.cluster.local:3000"
	ZarfInClusterArtifactServiceURL = ZarfInClusterGitServiceURL + "/api/packages/" + ZarfGitPushUser
)

// IPV6Localhost is the IP of localhost in IPv6 (TODO: move to helpers next to IPV4Localhost)
const IPV6Localhost = "::1"

// State is maintained as a secret in the Zarf namespace to track Zarf init data.
type State struct {
	// Indicates if Zarf was initialized while deploying its own k8s cluster
	ZarfAppliance bool `json:"zarfAppliance"`
	// K8s distribution of the cluster Zarf was deployed to
	Distro string `json:"distro"`
	// Machine architecture of the k8s node(s)
	Architecture string `json:"architecture"`
	// Default StorageClass value Zarf uses for variable templating
	StorageClass string `json:"storageClass"`
	// The IP family of the cluster, can be ipv4, ipv6, or dual
	IPFamily IPFamily `json:"ipFamily,omitempty"`
	// PKI certificate information for the agent pods Zarf manages
	AgentTLS     pki.GeneratedPKI `json:"agentTLS"`
	InjectorInfo InjectorInfo     `json:"injectorInfo"`

	// Information about the repository Zarf is configured to use
	GitServer GitServerInfo `json:"gitServer"`
	// Information about the container registry Zarf is configured to use
	RegistryInfo RegistryInfo `json:"registryInfo"`
	// Information about the artifact registry Zarf is configured to use
	ArtifactServer ArtifactServerInfo `json:"artifactServer"`
}

// InjectorInfo contains information on how to run the long lived Daemonset Injector
type InjectorInfo struct {
	// The image to be used for the long lived injector
	Image string `json:"injectorImage"`
	// The number of payload configmaps required
	PayLoadConfigMapAmount int `json:"payLoadConfigMapAmount"`
	// The PayLoadShaSum for the payload ConfigMaps
	PayLoadShaSum string `json:"payLoadShaSum"`
	// The port that the injector is exposed through, either hostPort or nodePort
	Port int `json:"port"`
}

// GitServerInfo contains information Zarf uses to communicate with a git repository to push/pull repositories to.
type GitServerInfo struct {
	// Username of a user with push access to the git repository
	PushUsername string `json:"pushUsername"`
	// Password of a user with push access to the git repository
	PushPassword string `json:"pushPassword"`
	// Username of a user with pull-only access to the git repository. If not provided for an external repository then the push-user is used
	PullUsername string `json:"pullUsername"`
	// Password of a user with pull-only access to the git repository. If not provided for an external repository then the push-user is used
	PullPassword string `json:"pullPassword"`
	// URL address of the git server
	Address string `json:"address"`
}

// IsInternal returns true if the git server URL is equivalent to a git server deployed through the default init package
func (gs GitServerInfo) IsInternal() bool {
	return gs.Address == ZarfInClusterGitServiceURL
}

// FillInEmptyValues sets every necessary value that's currently empty to a reasonable default
func (gs *GitServerInfo) FillInEmptyValues() error {
	var err error
	// Set default svc url if an external repository was not provided
	if gs.Address == "" {
		gs.Address = ZarfInClusterGitServiceURL
	}

	// Generate a push-user password if not provided by init flag
	if gs.PushPassword == "" {
		if gs.PushPassword, err = helpers.RandomString(ZarfGeneratedPasswordLen); err != nil {
			return fmt.Errorf("%s: %w", lang.ErrUnableToGenerateRandomSecret, err)
		}
	}

	if gs.PushUsername == "" && gs.IsInternal() {
		gs.PushUsername = ZarfGitPushUser
	}

	// Set read-user information if using an internal repository, otherwise copy from the push-user
	if gs.PullUsername == "" {
		if gs.IsInternal() {
			gs.PullUsername = ZarfGitReadUser
		} else {
			gs.PullUsername = gs.PushUsername
		}
	}
	if gs.PullPassword == "" {
		if gs.IsInternal() {
			if gs.PullPassword, err = helpers.RandomString(ZarfGeneratedPasswordLen); err != nil {
				return fmt.Errorf("%s: %w", lang.ErrUnableToGenerateRandomSecret, err)
			}
		} else {
			gs.PullPassword = gs.PushPassword
		}
	}

	return nil
}

// ArtifactServerInfo contains information Zarf uses to communicate with a artifact registry to push/pull repositories to.
type ArtifactServerInfo struct {
	// Username of a user with push access to the artifact registry
	PushUsername string `json:"pushUsername"`
	// Password of a user with push access to the artifact registry
	PushToken string `json:"pushPassword"`
	// URL address of the artifact registry
	Address string `json:"address"`
}

// IsInternal returns true if the artifact server URL is equivalent to the artifact server deployed through the default init package
func (as ArtifactServerInfo) IsInternal() bool {
	return as.Address == ZarfInClusterArtifactServiceURL
}

// FillInEmptyValues sets every necessary value that's currently empty to a reasonable default
func (as *ArtifactServerInfo) FillInEmptyValues() {
	// Set default svc url if an external registry was not provided
	if as.Address == "" {
		as.Address = ZarfInClusterArtifactServiceURL
	}

	// Set the push username to the git push user if not specified
	if as.PushUsername == "" {
		as.PushUsername = ZarfGitPushUser
	}
}

// RegistryMode defines how the registry is accessed
type RegistryMode string

const (
	// RegistryModeNodePort accesses the registry via NodePort service
	RegistryModeNodePort RegistryMode = "nodeport"
	// RegistryModeProxy accesses the registry via DaemonSet proxy
	RegistryModeProxy RegistryMode = "proxy"
)

// RegistryInfo contains information Zarf uses to communicate with a container registry to push/pull images.
type RegistryInfo struct {
	// Username of a user with push access to the registry
	PushUsername string `json:"pushUsername"`
	// Password of a user with push access to the registry
	PushPassword string `json:"pushPassword"`
	// Username of a user with pull-only access to the registry. If not provided for an external registry than the push-user is used
	PullUsername string `json:"pullUsername"`
	// Password of a user with pull-only access to the registry. If not provided for an external registry than the push-user is used
	PullPassword string `json:"pullPassword"`
	// URL address of the registry
	Address string `json:"address"`
	// Nodeport of the registry. Only needed if the internal Zarf registry is used and connected with over a nodeport service.
	NodePort int `json:"nodePort"`
	// Secret value that the registry was seeded with
	Secret string `json:"secret"`
	// RegistryMode defines how the registry is accessed (nodeport or proxy)
	RegistryMode RegistryMode `json:"registryMode"`
}

// CheckIfCredsChanged compares two RegistryInfo structs and returns true if any non-empty fields have changed
func CheckIfCredsChanged(existing, given RegistryInfo) bool {
	if given.PushUsername != "" && existing.PushUsername != given.PushUsername {
		return true
	}
	if given.PullUsername != "" && existing.PullUsername != given.PullUsername {
		return true
	}
	if given.PushPassword != "" && existing.PushPassword != given.PushPassword {
		return true
	}
	if given.PullPassword != "" && existing.PullPassword != given.PullPassword {
		return true
	}
	if given.Address != "" && existing.Address != given.Address {
		return true
	}
	if given.Secret != "" && existing.Secret != given.Secret {
		return true
	}
	return false
}

// IsInternal returns true if the registry URL is equivalent to the registry deployed through the default init package
func (ri RegistryInfo) IsInternal() bool {
	return ri.Address == fmt.Sprintf("%s:%d", helpers.IPV4Localhost, ri.NodePort) ||
		ri.Address == fmt.Sprintf("[%s]:%d", IPV6Localhost, ri.NodePort)
}

// FillInEmptyValues sets every necessary value not already set to a reasonable default
func (ri *RegistryInfo) FillInEmptyValues(ipFamily IPFamily) error {
	var err error

	if ri.RegistryMode == "" {
		ri.RegistryMode = RegistryModeNodePort
	}
	// Set default NodePort if none was provided and the registry is internal
	if ri.NodePort == 0 && ri.Address == "" {
		switch ri.RegistryMode {
		case RegistryModeNodePort:
			ri.NodePort = ZarfInClusterContainerRegistryNodePort
		// In proxy mode, we should avoid using a port in the nodeport range as Kubernetes will still randomly assign nodeports even on already claimed hostports
		case RegistryModeProxy:
			ri.NodePort = ZarfRegistryHostPort
		}
	}

	// Set default url if an external registry was not provided
	if ri.Address == "" {
		ri.Address = LocalhostRegistryAddress(ipFamily, ri.NodePort)
	}

	// Generate a push-user password if not provided by init flag
	if ri.PushPassword == "" {
		if ri.PushPassword, err = helpers.RandomString(ZarfGeneratedPasswordLen); err != nil {
			return fmt.Errorf("%s: %w", lang.ErrUnableToGenerateRandomSecret, err)
		}
	}

	if ri.PushUsername == "" && ri.IsInternal() {
		ri.PushUsername = ZarfRegistryPushUser
	}

	// Set pull-username if not provided by init flag
	if ri.PullUsername == "" {
		if ri.IsInternal() {
			ri.PullUsername = ZarfRegistryPullUser
		} else {
			// If this is an external registry and a pull-user wasn't provided, use the same credentials as the push user
			ri.PullUsername = ri.PushUsername
		}
	}
	if ri.PullPassword == "" {
		if ri.IsInternal() {
			if ri.PullPassword, err = helpers.RandomString(ZarfGeneratedPasswordLen); err != nil {
				return fmt.Errorf("%s: %w", lang.ErrUnableToGenerateRandomSecret, err)
			}
		} else {
			// If this is an external registry and a pull-user wasn't provided, use the same credentials as the push user
			ri.PullPassword = ri.PushPassword
		}
	}

	if ri.Secret == "" {
		if ri.Secret, err = helpers.RandomString(ZarfGeneratedSecretLen); err != nil {
			return fmt.Errorf("%s: %w", lang.ErrUnableToGenerateRandomSecret, err)
		}
	}

	return nil
}

// Default returns a default State with default values filled in for the registry, git server, and artifact server
func Default() (*State, error) {
	state := &State{}
	err := state.GitServer.FillInEmptyValues()
	if err != nil {
		return nil, err
	}
	err = state.RegistryInfo.FillInEmptyValues(IPFamilyDualStack)
	if err != nil {
		return nil, err
	}
	state.ArtifactServer.FillInEmptyValues()
	return state, nil
}

// MergeOptions tracks the user-defined options during cluster initialization.
// TODO(mkcp): Provide semantic doccomments for how exported fields are used.
type MergeOptions struct {
	GitServer      GitServerInfo
	RegistryInfo   RegistryInfo
	ArtifactServer ArtifactServerInfo
	Services       []string
}

// Merge merges init options for provided services into the provided state to create a new state struct
func Merge(oldState *State, opts MergeOptions) (*State, error) {
	newState := *oldState
	var err error
	if slices.Contains(opts.Services, RegistryKey) {
		// TODO: Replace use of reflections with explicit setting
		newState.RegistryInfo = helpers.MergeNonZero(newState.RegistryInfo, opts.RegistryInfo)

		// Set the new passwords if they should be autogenerated
		if newState.RegistryInfo.PushPassword == oldState.RegistryInfo.PushPassword && oldState.RegistryInfo.IsInternal() {
			if newState.RegistryInfo.PushPassword, err = helpers.RandomString(ZarfGeneratedPasswordLen); err != nil {
				return nil, fmt.Errorf("%s: %w", lang.ErrUnableToGenerateRandomSecret, err)
			}
		}
		if newState.RegistryInfo.PullPassword == oldState.RegistryInfo.PullPassword && oldState.RegistryInfo.IsInternal() {
			if newState.RegistryInfo.PullPassword, err = helpers.RandomString(ZarfGeneratedPasswordLen); err != nil {
				return nil, fmt.Errorf("%s: %w", lang.ErrUnableToGenerateRandomSecret, err)
			}
		}
	}
	if slices.Contains(opts.Services, GitKey) {
		// TODO: Replace use of reflections with explicit setting
		newState.GitServer = helpers.MergeNonZero(newState.GitServer, opts.GitServer)

		// Set the new passwords if they should be autogenerated
		if newState.GitServer.PushPassword == oldState.GitServer.PushPassword && oldState.GitServer.IsInternal() {
			if newState.GitServer.PushPassword, err = helpers.RandomString(ZarfGeneratedPasswordLen); err != nil {
				return nil, fmt.Errorf("%s: %w", lang.ErrUnableToGenerateRandomSecret, err)
			}
		}
		if newState.GitServer.PullPassword == oldState.GitServer.PullPassword && oldState.GitServer.IsInternal() {
			if newState.GitServer.PullPassword, err = helpers.RandomString(ZarfGeneratedPasswordLen); err != nil {
				return nil, fmt.Errorf("%s: %w", lang.ErrUnableToGenerateRandomSecret, err)
			}
		}
	}
	if slices.Contains(opts.Services, ArtifactKey) {
		// TODO: Replace use of reflections with explicit setting
		newState.ArtifactServer = helpers.MergeNonZero(newState.ArtifactServer, opts.ArtifactServer)

		// Set an empty token if it should be autogenerated
		if newState.ArtifactServer.PushToken == oldState.ArtifactServer.PushToken && oldState.ArtifactServer.IsInternal() {
			newState.ArtifactServer.PushToken = ""
		}
	}
	if slices.Contains(opts.Services, AgentKey) {
		agentTLS, err := pki.GeneratePKI(ZarfAgentHost)
		if err != nil {
			return nil, err
		}
		newState.AgentTLS = agentTLS
	}

	return &newState, nil
}

// DebugPrint takes a State struct, sanitizes sensitive fields, and prints them.
func DebugPrint(ctx context.Context, state *State) {
	if state == nil {
		return
	}
	// this is a shallow copy, nested pointers WILL NOT be copied
	oldState := *state
	sanitized := sanitizeState(&oldState)
	logger.From(ctx).Debug("debugPrintZarfState", "state", sanitized)
}

func sanitizeState(s *State) *State {
	// Overwrite the AgentTLS information
	s.AgentTLS.CA = []byte("**sanitized**")
	s.AgentTLS.Cert = []byte("**sanitized**")
	s.AgentTLS.Key = []byte("**sanitized**")

	// Overwrite the GitServer passwords
	s.GitServer.PushPassword = "**sanitized**"
	s.GitServer.PullPassword = "**sanitized**"

	// Overwrite the RegistryInfo passwords
	s.RegistryInfo.PushPassword = "**sanitized**"
	s.RegistryInfo.PullPassword = "**sanitized**"
	s.RegistryInfo.Secret = "**sanitized**"

	// Overwrite the ArtifactServer secret
	s.ArtifactServer.PushToken = "**sanitized**"

	return s
}

// DeployedPackageOptions are options for the DeployedPackage function
type DeployedPackageOptions func(*DeployedPackage)

// WithPackageNamespaceOverride sets the [ALPHA] optional namespace override for a package during deployment
func WithPackageNamespaceOverride(namespaceOverride string) DeployedPackageOptions {
	return func(o *DeployedPackage) {
		o.NamespaceOverride = namespaceOverride
	}
}

// DeployedPackage contains information about a Zarf Package that has been deployed to a cluster
// This object is saved as the data of a k8s secret within the 'Zarf' namespace (not as part of the ZarfState secret).
type DeployedPackage struct {
	Name               string               `json:"name"`
	Data               v1alpha1.ZarfPackage `json:"data"`
	CLIVersion         string               `json:"cliVersion"`
	Generation         int                  `json:"generation"`
	DeployedComponents []DeployedComponent  `json:"deployedComponents"`
	ConnectStrings     ConnectStrings       `json:"connectStrings,omitempty"`
	// [ALPHA] Optional namespace override - exported/json-tag for storage in deployed package state secret
	NamespaceOverride string `json:"namespaceOverride,omitempty"`
}

// GetSecretName returns the k8s secret name for the deployed package
func (d *DeployedPackage) GetSecretName() string {
	if d.NamespaceOverride != "" {
		// override keyword is used to help prevent collisions in secret name
		return fmt.Sprintf("%s-%s-override-%s", "zarf-package", d.Name, d.NamespaceOverride)
	}
	return fmt.Sprintf("%s-%s", "zarf-package", d.Name)
}

// ConnectString contains information about a connection made with Zarf connect.
type ConnectString struct {
	// Descriptive text that explains what the resource you would be connecting to is used for
	Description string `json:"description"`
	// URL path that gets appended to the k8s port-forward result
	URL string `json:"url"`
}

// ConnectStrings is a map of connect names to connection information.
type ConnectStrings map[string]ConnectString

// DeployedComponent contains information about a Zarf Package Component that has been deployed to a cluster.
type DeployedComponent struct {
	Name               string           `json:"name"`
	InstalledCharts    []InstalledChart `json:"installedCharts"`
	Status             ComponentStatus  `json:"status"`
	ObservedGeneration int              `json:"observedGeneration"`
}

// ChartStatus is the status of a Helm Chart release
type ChartStatus string

// InstalledChart contains information about a Helm Chart that has been deployed to a cluster.
type InstalledChart struct {
	Namespace      string         `json:"namespace"`
	ChartName      string         `json:"chartName"`
	ConnectStrings ConnectStrings `json:"connectStrings,omitempty"`
	Status         ChartStatus    `json:"status"`
}

// MergeInstalledChartsForComponent merges the provided existing charts with the provided installed charts.
func MergeInstalledChartsForComponent(existingCharts, installedCharts []InstalledChart, partial bool) []InstalledChart {
	key := func(chart InstalledChart) string {
		return fmt.Sprintf("%s/%s", chart.Namespace, chart.ChartName)
	}

	lookup := make(map[string]InstalledChart, 0)
	for _, chart := range existingCharts {
		lookup[key(chart)] = chart
	}

	// Track which keys are still present in newCharts
	seen := make(map[string]struct{}, len(installedCharts)+len(existingCharts))

	for _, chart := range installedCharts {
		k := key(chart)
		seen[k] = struct{}{}

		if _, ok := lookup[k]; ok {
			existingChart := lookup[k]
			existingChart.ConnectStrings = chart.ConnectStrings
			existingChart.Status = chart.Status
			lookup[k] = existingChart
		} else {
			lookup[k] = chart
		}
	}

	// retain existing charts that are no longer present if not a partial
	if !partial {
		for k, chart := range lookup {
			if _, ok := seen[k]; !ok {
				lookup[k] = chart
			}
		}
	}

	merged := make([]InstalledChart, 0, len(lookup))
	for _, chart := range lookup {
		merged = append(merged, chart)
	}

	return merged
}

// LocalhostRegistryAddress builds the IPv4 or IPv6 local address of the Zarf deployed registry.
func LocalhostRegistryAddress(ipFamily IPFamily, nodePort int) string {
	if ipFamily == IPFamilyIPv6 {
		return fmt.Sprintf("[%s]:%d", IPV6Localhost, nodePort)
	}
	return fmt.Sprintf("%s:%d", helpers.IPV4Localhost, nodePort)
}
