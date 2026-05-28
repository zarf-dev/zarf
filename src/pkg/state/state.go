// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package state manages references to a logical zarf deployment in k8s.
package state

import (
	"context"
	"fmt"
	"regexp"

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

// ServiceKey identifies a Zarf-managed service in state (registry, git, artifact, agent).
type ServiceKey string

// Credential keys
const (
	RegistryKey ServiceKey = "registry"
	GitKey      ServiceKey = "git"
	ArtifactKey ServiceKey = "artifact"
	AgentKey    ServiceKey = "agent"
)

// AllServiceKeys is the canonical ordered list of every supported service key.
func AllServiceKeys() []ServiceKey {
	return []ServiceKey{RegistryKey, GitKey, ArtifactKey, AgentKey}
}

// ServiceSet is an unordered set of ServiceKeys.
type ServiceSet map[ServiceKey]struct{}

// NewServiceSet returns a ServiceSet populated with the given keys.
func NewServiceSet(keys ...ServiceKey) ServiceSet {
	s := make(ServiceSet, len(keys))
	for _, k := range keys {
		s[k] = struct{}{}
	}
	return s
}

// Has reports whether k is in the set.
func (s ServiceSet) Has(k ServiceKey) bool {
	_, ok := s[k]
	return ok
}

// Add inserts k into the set.
func (s ServiceSet) Add(k ServiceKey) {
	s[k] = struct{}{}
}

// ParseServiceKey returns the ServiceKey matching s, or an error if s is not recognized.
func ParseServiceKey(s string) (ServiceKey, error) {
	for _, k := range AllServiceKeys() {
		if string(k) == s {
			return k, nil
		}
	}
	return "", fmt.Errorf("invalid service key %q, valid keys are: %v", s, AllServiceKeys())
}

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
	ZarfInjectorDefaultHostPort            = 5001
	ZarfRegistryHostPort                   = 5000
	ZarfRegistryPushUser                   = "zarf-push"
	ZarfRegistryPullUser                   = "zarf-pull"

	ZarfGitPushUser = "zarf-git-user"
	ZarfGitReadUser = "zarf-git-read-user"
	ZarfAgentHost   = "agent-hook.zarf.svc"

	ZarfInClusterGitServiceURL      = "http://zarf-gitea-http.zarf.svc.cluster.local:3000"
	ZarfInClusterArtifactServiceURL = ZarfInClusterGitServiceURL + "/api/packages/" + ZarfGitPushUser

	// ZarfRegistryMTLSServerCommonName is the common name for the registry server certificate
	ZarfRegistryMTLSServerCommonName = "zarf-docker-registry"
	// ZarfRegistryMTLSClientCommonName is the common name for the registry client certificate
	ZarfRegistryMTLSClientCommonName = "zarf-registry-client"
	ZarfRegistryMTLSCASubject        = "Zarf Registry CA"
)

// ZarfRegistryMTLSServerHosts is the list of DNS names and IPs for the registry server certificate
var ZarfRegistryMTLSServerHosts = []string{
	"zarf-docker-registry",
	"zarf-docker-registry.zarf.svc.cluster.local",
	"localhost",
	"127.0.0.1",
	"[::1]",
}

// IPV6Localhost is the IP of localhost in IPv6 (TODO: move to helpers next to IPV4Localhost)
const IPV6Localhost = "::1"

// State is maintained as a secret in the Zarf namespace to track Zarf init data.
type State struct {
	// Indicates if Zarf was initialized while deploying its own k8s cluster
	ZarfAppliance bool `json:"zarfAppliance"`
	// K8s distribution of the cluster Zarf was deployed to
	Distro string `json:"distro"`
	// Default StorageClass value Zarf uses for variable templating
	StorageClass string `json:"storageClass"`
	// The IP family of the cluster, can be ipv4, ipv6, or dual
	IPFamily IPFamily `json:"ipFamily,omitempty"`
	// PKI certificate information for the agent pods Zarf manages
	AgentTLS pki.GeneratedPKI `json:"agentTLS"`
	// AgentTLSUserProvided indicates whether the agent TLS certs were provided by the user rather than auto-generated
	AgentTLSUserProvided bool         `json:"agentTLSUserProvided,omitempty"`
	InjectorInfo         InjectorInfo `json:"injectorInfo"`

	// Information about the repository Zarf is configured to use
	GitServer GitServerInfo `json:"gitServer"`
	// Information about the container registry Zarf is configured to use
	RegistryInfo RegistryInfo `json:"registryInfo"`
	// Information about the artifact registry Zarf is configured to use
	ArtifactServer ArtifactServerInfo `json:"artifactServer"`
}

// AgentIsConfigured returns true when Zarf has agent TLS configured.
func (s *State) AgentIsConfigured() bool {
	return len(s.AgentTLS.Cert) > 0
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

// IsConfigured returns true if the git server address has been set.
// clusters initialized before services-gated state https://github.com/zarf-dev/zarf/pull/4832
// may report true even without a real git server.
func (gs GitServerInfo) IsConfigured() bool {
	return gs.Address != ""
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

// IsConfigured returns true if the artifact server address has been set.
func (as ArtifactServerInfo) IsConfigured() bool {
	return as.Address != ""
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

// MTLSStrategy defines the strategy to manage the mTLS certificates for the registry
type MTLSStrategy string

const (
	// MTLSStrategyNone indicates no mTLS certificate management
	MTLSStrategyNone MTLSStrategy = "none"
	// MTLSStrategyZarfManaged indicates Zarf is managing the mTLS certificates
	MTLSStrategyZarfManaged MTLSStrategy = "zarf-managed"
)

// RegistryMode defines how the registry is accessed
type RegistryMode string

const (
	// RegistryModeNodePort accesses the registry via NodePort service
	RegistryModeNodePort RegistryMode = "nodeport"
	// RegistryModeProxy accesses the registry via DaemonSet proxy
	RegistryModeProxy RegistryMode = "proxy"
	// RegistryModeExternal is used when the user has an external registry
	RegistryModeExternal RegistryMode = "external"
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
	// Deprecated: Use Port instead. Kept for backwards compatibility with state JSON written by older Zarf versions.
	NodePort int `json:"nodePort"`
	// Port of the internal registry. In nodeport mode this is a Kubernetes NodePort, in proxy mode it is a host port.
	Port int `json:"port"`
	// Secret value that the registry was seeded with
	Secret string `json:"secret"`
	// RegistryMode defines how the registry is accessed (nodeport, proxy, or external)
	RegistryMode RegistryMode `json:"registryMode"`
	// MTLSStrategy defines who manages the mTLS certificates for the registry (defaults to none)
	MTLSStrategy MTLSStrategy `json:"mtlsStrategy,omitempty"`
}

// ReconcilePort syncs the deprecated NodePort field with Port at serialization boundaries.
// On read (LoadState): copies NodePort into Port when Port is unset, for state written by older Zarf.
// On write (SaveState): copies Port into NodePort so older Zarf versions can read the state.
func (ri *RegistryInfo) ReconcilePort() {
	if ri.Port == 0 && ri.NodePort != 0 {
		ri.Port = ri.NodePort
	}
	ri.NodePort = ri.Port
}

// IsInternal returns true if the registry URL is equivalent to the registry deployed through the default init package
func (ri RegistryInfo) IsInternal() bool {
	if ri.RegistryMode != "" {
		return ri.RegistryMode != RegistryModeExternal
	}
	// This is kept for backwards compatibility with previous versions of Zarf that did not set the registry mode
	return ri.Address == fmt.Sprintf("%s:%d", helpers.IPV4Localhost, ri.Port) ||
		ri.Address == fmt.Sprintf("[%s]:%d", IPV6Localhost, ri.Port)
}

// IsConfigured returns true if the registry info address has been set
func (ri RegistryInfo) IsConfigured() bool {
	return ri.Address != ""
}

// ShouldUseMTLS returns true if mTLS should be used for the registry connection.
func (ri RegistryInfo) ShouldUseMTLS() bool {
	return ri.MTLSStrategy != "" && ri.MTLSStrategy != MTLSStrategyNone
}

// CheckIfRegistryAddressOrCredsChanged compares two RegistryInfo structs and returns true if the creds or address changed
func CheckIfRegistryAddressOrCredsChanged(existing, given RegistryInfo) bool {
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

// FillInEmptyValues sets every necessary value not already set to a reasonable default
func (ri *RegistryInfo) FillInEmptyValues(ipFamily IPFamily) error {
	var err error

	// If registry mode is empty, then default to nodeport if internal, or set as external if address is set
	if ri.RegistryMode == "" {
		if ri.Address == "" {
			ri.RegistryMode = RegistryModeNodePort
		} else {
			ri.RegistryMode = RegistryModeExternal
		}
	}

	if ri.Port == 0 && ri.Address == "" {
		switch ri.RegistryMode {
		// Set default port if none was provided and the registry is internal
		case RegistryModeNodePort:
			ri.Port = ZarfInClusterContainerRegistryNodePort
		// In proxy mode, we should avoid using a port in the nodeport range as Kubernetes will still randomly assign nodeports even on already claimed hostports
		case RegistryModeProxy:
			ri.Port = ZarfRegistryHostPort
		}
	}

	// Set default url if an external registry was not provided
	if ri.Address == "" {
		ri.Address = LocalhostRegistryAddress(ipFamily, ri.Port)
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

	if ri.MTLSStrategy == "" {
		ri.MTLSStrategy = MTLSStrategyNone
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
	Services       ServiceSet
	// AgentTLS allows providing user-managed TLS certificates for the agent. When nil, certs are auto-generated.
	AgentTLS *pki.GeneratedPKI
}

// Merge merges init options for provided services into the provided state to create a new state struct
func Merge(oldState *State, opts MergeOptions) (*State, error) {
	newState := *oldState
	var err error
	if opts.Services.Has(RegistryKey) {
		// TODO: Replace use of reflections with explicit setting
		newState.RegistryInfo = helpers.MergeNonZero(newState.RegistryInfo, opts.RegistryInfo)

		// Only autogenerate passwords if the user didn't provide one and the registry is internal
		if opts.RegistryInfo.PushPassword == "" && oldState.RegistryInfo.IsInternal() {
			if newState.RegistryInfo.PushPassword, err = helpers.RandomString(ZarfGeneratedPasswordLen); err != nil {
				return nil, fmt.Errorf("%s: %w", lang.ErrUnableToGenerateRandomSecret, err)
			}
		}
		if opts.RegistryInfo.PullPassword == "" && oldState.RegistryInfo.IsInternal() {
			if newState.RegistryInfo.PullPassword, err = helpers.RandomString(ZarfGeneratedPasswordLen); err != nil {
				return nil, fmt.Errorf("%s: %w", lang.ErrUnableToGenerateRandomSecret, err)
			}
		}
	}
	if opts.Services.Has(GitKey) {
		// TODO: Replace use of reflections with explicit setting
		newState.GitServer = helpers.MergeNonZero(newState.GitServer, opts.GitServer)

		// Only autogenerate passwords if the user didn't provide one and the git server is internal
		if opts.GitServer.PushPassword == "" && oldState.GitServer.IsInternal() {
			if newState.GitServer.PushPassword, err = helpers.RandomString(ZarfGeneratedPasswordLen); err != nil {
				return nil, fmt.Errorf("%s: %w", lang.ErrUnableToGenerateRandomSecret, err)
			}
		}
		if opts.GitServer.PullPassword == "" && oldState.GitServer.IsInternal() {
			if newState.GitServer.PullPassword, err = helpers.RandomString(ZarfGeneratedPasswordLen); err != nil {
				return nil, fmt.Errorf("%s: %w", lang.ErrUnableToGenerateRandomSecret, err)
			}
		}
	}
	if opts.Services.Has(ArtifactKey) {
		// TODO: Replace use of reflections with explicit setting
		newState.ArtifactServer = helpers.MergeNonZero(newState.ArtifactServer, opts.ArtifactServer)

		// Only clear token for autogeneration if the user didn't provide one and the artifact server is internal
		if opts.ArtifactServer.PushToken == "" && oldState.ArtifactServer.IsInternal() {
			newState.ArtifactServer.PushToken = ""
		}
	}
	if opts.Services.Has(AgentKey) {
		if opts.AgentTLS != nil {
			newState.AgentTLS = *opts.AgentTLS
			newState.AgentTLSUserProvided = true
		} else {
			agentTLS, err := pki.GeneratePKI(ZarfAgentHost)
			if err != nil {
				return nil, err
			}
			newState.AgentTLS = agentTLS
			newState.AgentTLSUserProvided = false
		}
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

// WithPackageConnectivity sets the connectivity mode for the deployed package
func WithPackageConnectivity(connected bool) DeployedPackageOptions {
	return func(o *DeployedPackage) {
		if connected {
			o.PackageConnectivity = PackageConnectivityConnected
		} else {
			o.PackageConnectivity = PackageConnectivityAirGap
		}
	}
}

// PackageConnectivity defines the connectivity mode of package deployments
type PackageConnectivity string

const (
	// PackageConnectivityAirGap is the default deploy mode
	PackageConnectivityAirGap PackageConnectivity = "airgap"
	// PackageConnectivityConnected is used when a package is deployed with YOLO or in connected mode.
	PackageConnectivityConnected PackageConnectivity = "connected"
)

// DeployedPackage contains information about a Zarf Package that has been deployed to a cluster
// This object is saved as the data of a k8s secret within the 'Zarf' namespace (not as part of the ZarfState secret).
type DeployedPackage struct {
	Name                string               `json:"name"`
	Data                v1alpha1.ZarfPackage `json:"data"`
	CLIVersion          string               `json:"cliVersion"`
	Generation          int                  `json:"generation"`
	DeployedComponents  []DeployedComponent  `json:"deployedComponents"`
	ConnectStrings      ConnectStrings       `json:"connectStrings,omitempty"`
	PackageConnectivity PackageConnectivity  `json:"packageConnectivity"`
	// [ALPHA] Optional namespace override - exported/json-tag for storage in deployed package state secret
	NamespaceOverride string `json:"namespaceOverride,omitempty"`
}

// DeployedPackageNameRegex is a regex for lowercase, numbers and hyphens that cannot start with a hyphen.
// https://regex101.com/r/FLdG9G/2
var DeployedPackageNameRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9\-]*$`).MatchString

// GetSecretName returns the k8s secret name for the deployed package
func (d *DeployedPackage) GetSecretName() string {
	if d.NamespaceOverride != "" {
		// override keyword is used to help prevent collisions in secret name
		return fmt.Sprintf("%s-%s-override-%s", "zarf-package", d.Name, d.NamespaceOverride)
	}
	return fmt.Sprintf("%s-%s", "zarf-package", d.Name)
}

// GetPackageConnectivity returns the connectivity mode the package is using
// Defaults to airgap for packages that were deployed before connectivity was introduced
func (d *DeployedPackage) GetPackageConnectivity() PackageConnectivity {
	if d.PackageConnectivity == "" {
		return PackageConnectivityAirGap
	}
	return d.PackageConnectivity
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
func LocalhostRegistryAddress(ipFamily IPFamily, port int) string {
	if ipFamily == IPFamilyIPv6 {
		return fmt.Sprintf("[%s]:%d", IPV6Localhost, port)
	}
	return fmt.Sprintf("%s:%d", helpers.IPV4Localhost, port)
}
