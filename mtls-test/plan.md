Based on my analysis of the codebase and the mtls-test scripts, here's a comprehensive plan for implementing mTLS for your registry when registryProxy mode is true
  during zarf init:

  mTLS Implementation Plan for Registry Proxy Mode

  1. Certificate Generation Strategy

  - Self-signed CA approach (following mtls-test/script.sh pattern):
    - Generate CA private key and certificate
    - Create server certificates for registry with SAN entries for zarf-docker-registry, localhost, 127.0.0.1
    - Create client certificates for proxy with client authentication usage
    - Store certificates as Kubernetes secrets in zarf namespace

  2. Integration Points in Zarf Init Flow

  Key locations to modify:

  - initialize.go:58 - Add new flag --registry-mtls to enable mTLS when --registry-proxy is used
  - pkg/packager/deploy.go:301-323 - Extend the registryProxy mode logic to include certificate generation
  - pkg/state/state.go:102 - Add mTLS configuration to RegistryInfo struct
  - pkg/cluster/injector.go - Modify injector setup to handle mTLS certificates

  3. Implementation Steps

  Phase 1: State and Configuration

  1. Extend RegistryInfo struct with mTLS fields:
  type RegistryInfo struct {
      // existing fields...
      MTLSEnabled bool `json:"mtlsEnabled"`
      CACertPath  string `json:"caCertPath,omitempty"`
  }
  2. Add validation in validateInitFlags() to ensure mTLS is only used with proxy mode

  Phase 2: Certificate Management

  1. Create new package pkg/pki/registry with functions:
    - GenerateRegistryMTLSCerts() - Generate CA, server, and client certificates
    - CreateRegistryMTLSSecrets() - Create Kubernetes secrets for certificates
    - Follow existing pkg/pki patterns used for AgentTLS

  Phase 3: Registry Proxy Integration

  1. Modify deployComponent() logic in deploy.go:
    - When isSeedRegistry && d.s.RegistryInfo.ProxyMode && d.s.RegistryInfo.MTLSEnabled
    - Generate certificates before creating injector config maps
    - Store certificate info in state for later use
  2. Update injector DaemonSet configuration to:
    - Mount mTLS certificates as volumes
    - Configure registry and proxy containers with TLS settings

  Phase 4: Runtime Configuration

  1. Modify registry deployment templates to:
    - Enable TLS on registry server with server certificates
    - Configure client certificate verification
  2. Update proxy DaemonSet to:
    - Use client certificates for registry communication
    - Verify server certificates against CA

  4. File Structure Changes

  New files to create:
  - src/pkg/pki/registry/mtls.go - Certificate generation logic
  - src/pkg/pki/registry/mtls_test.go - Unit tests

  Files to modify:
  - src/cmd/initialize.go - Add --registry-mtls flag
  - src/pkg/state/state.go - Extend RegistryInfo struct
  - src/pkg/packager/deploy.go - Integrate mTLS setup
  - packages/zarf-registry/chart/templates/ - Update registry/proxy configs

  5. Security Considerations

  - Certificates stored as Kubernetes TLS secrets with proper RBAC
  - CA certificate expiry handling (365 days default, make configurable)
  - Certificate rotation strategy for long-running clusters
  - Proper cleanup on zarf destroy

  6. Testing Strategy

  - Unit tests for certificate generation functions
  - Integration tests verifying mTLS handshake between proxy and registry
  - Validation that non-mTLS proxy mode continues to work unchanged

  This approach follows the existing Zarf patterns for certificate management (like AgentTLS) while implementing the mTLS functionality demonstrated in your test
  scripts.
