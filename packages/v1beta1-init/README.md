# v1beta1 init package

This init package uses `zarf.dev/v1beta1`, separate `ZarfComponentConfig` definitions,
and structured deployment values. It reuses the existing registry and agent charts,
Gitea chart, connect manifests, and K3s cleanup script. K3s and the Git server are
optional; the injector, seed registry, registry, and agent provide the required init
services.

The component directories are `injector/`, `agent/`, `registry/`, `git-server/`,
and `k3s/`. K3s and the injector have separate `amd64.tpl.yaml` and `arm64.tpl.yaml`
variants; K3s imports shared files, values, and lifecycle actions from `common.tpl.yaml`.
The seed registry and shared registry base live under `registry/`.

Build the CLI from this branch before using these targets. v1beta1 and
`zarf dev template` are still under development. This package requires a CLI with
v1beta1 init services, component imports, state access, and the Gitea helper's
`--pvc-name` flag, plus the `imageRepository` and `imageTagOrDigest` template functions.
Older CLIs should continue using the root init package and
`make init-package`. The existing `release-init-package` target continues building
the legacy init package for its current release workflows.

## Build

From the repository root:

```sh
make build
make init-package-v1beta1
```

The target rebuilds the CLI and local agent image for `ARCH`, renders `zarf.tpl.yaml`
and `{injector,agent,registry,git-server,k3s}/*.tpl.yaml` with `zarf dev template`,
and creates the package with SBOMs under `build/v1beta1-init`. This target always
builds the local agent and requires Docker.
Generated `*.gen.yaml` definitions are ignored by Git and regenerated on every build.
The package and component versions come from `[[ .cli.version ]]`.

Render definitions without creating the package:

```sh
make template-init-package-v1beta1 ARCH=arm64
```

`ARCH` supports `amd64` and `arm64`. Rendering a second architecture replaces the
previous generated definitions. Run builds for different architectures sequentially.
`INIT_V1BETA1_OUTPUT` changes the package output directory.

Create a package using a released agent image without building the local agent:

```sh
make release-init-package-v1beta1 AGENT_IMAGE_TAG=v0.87.0
```

This target defaults `agent.image` to `ghcr.io/zarf-dev/zarf/agent:<AGENT_IMAGE_TAG>`
and `agent.source` to `registry`. It requires `AGENT_IMAGE_TAG` and creates the
package with SBOMs under `INIT_V1BETA1_OUTPUT`. It does not publish anything.

Build-time image settings come from `template-values.yaml`. Override dotted keys
through `INIT_V1BETA1_TEMPLATE_SET`, or supply a complete values file through
`INIT_V1BETA1_TEMPLATE_VALUES`. The release target applies its agent defaults after
the values file, followed by `INIT_V1BETA1_TEMPLATE_SET` overrides:

```sh
make release-init-package-v1beta1 AGENT_IMAGE_TAG=v0.87.0 \
  INIT_V1BETA1_TEMPLATE_SET='registry.image=docker.io/library/registry:3.1.1'
```

The agent, registry, and proxy accept tagged or digest-pinned images. Their full
chart references are derived from the packaged images using `imageRepository` and
`imageTagOrDigest`, including registry ports and Docker Hub shorthand. Build-time
overrides also affect deployment; a digest takes precedence when a reference
contains both a tag and a digest. Set `agent.source=registry` for a released image;
`agent.source=daemon` reads a locally available Docker image.
`init-package-v1beta1` always builds `ghcr.io/zarf-dev/zarf/agent:local`; the release
target uses the selected image without building it.

The v1beta1 targets use `zarf dev template` settings rather than the legacy
`[package.create.set]` settings in `zarf-config.toml`. The release target maps
`AGENT_IMAGE_TAG` to the agent template value.

The reused agent and registry charts accept an optional `image.reference` with a
complete image reference. The registry chart also accepts `proxy.image.reference`
and `proxy.registry.image.reference`. When the proxy is enabled, the latter
overrides `image.reference` to select the seed registry. Empty reference fields
retain the existing repository/tag behavior, including the proxy registry
repository override. Existing chart consumers require no migration.

## Deployment values

Pass `--features=values=true` when inspecting or deploying this package. Use
`--values` for structured overrides and `--set-values` for individual settings.
Defaults live in `*-values.yaml` files in each component directory. Chart settings
map from `.agent`, `.registry`, and `.gitServer` directly to Helm values.
K3s uses `.k3s.args` in its templated systemd service.

For example, save this as a deployment values file:

```yaml
registry:
  persistence:
    size: 40Gi
  proxy:
    hostNetwork: true
agent:
  nodeSelector:
    role: infra
  tolerations:
    - key: dedicated
      operator: Equal
      value: infra
      effect: NoSchedule
gitServer:
  persistence:
    claimName: data-zarf-gitea-0
```

Migration examples from legacy variables:

| Legacy variable | Deployment value |
| --- | --- |
| `K3S_ARGS` | `k3s.args` |
| `AGENT_AFFINITY`, `AGENT_TOLERATIONS`, `AGENT_NODE_SELECTOR` | `agent.affinity`, `agent.tolerations`, `agent.nodeSelector` |
| `AGENT_WEBHOOK_ANNOTATIONS`, `AGENT_MUTATION_EXCLUSIONS` | `agent.webhookAnnotations`, `agent.mutationExclusions` |
| `REGISTRY_EXISTING_PVC`, `REGISTRY_PVC_SIZE`, `REGISTRY_PVC_ENABLED` | `registry.persistence.existingClaim`, `registry.persistence.size`, `registry.persistence.enabled` |
| `REGISTRY_PVC_ACCESS_MODE` | `registry.persistence.accessMode` |
| `REGISTRY_CPU_REQ`, `REGISTRY_MEM_LIMIT` | `registry.resources.requests.cpu`, `registry.resources.limits.memory` |
| `REGISTRY_HPA_ENABLE`, `REGISTRY_HPA_MIN`, `REGISTRY_HPA_MAX` | `registry.autoscaling.enabled`, `registry.autoscaling.minReplicas`, `registry.autoscaling.maxReplicas` |
| `REGISTRY_HPA_AUTO_SIZE`, `REGISTRY_HPA_TARGET_CPU` | `registry.autoscaling.mapReplicasToNodes`, `registry.autoscaling.targetCPUUtilizationPercentage` |
| `REGISTRY_CA_BUNDLE`, `REGISTRY_EXTRA_ENVS` | `registry.caBundle` (certificate contents), `registry.extraEnvVars` (array) |
| `REGISTRY_AFFINITY_ENABLE`, `REGISTRY_AFFINITY_CUSTOM` | `registry.affinity.enabled`, `registry.affinity.custom` |
| `PROXY_TOLERATIONS`, `HOST_NETWORK_PROXY` | `registry.proxy.tolerations`, `registry.proxy.hostNetwork` |
| `GIT_SERVER_EXISTING_PVC`, `GIT_SERVER_PVC_SIZE` | `gitServer.persistence.claimName`, `gitServer.persistence.size` |
| `GIT_SERVER_PVC_ACCESS_MODE` | `gitServer.persistence.accessModes` (array) |
| `GIT_SERVER_REPLICA_COUNT`, `GIT_SERVER_DISABLE_REGISTRATION` | `gitServer.replicaCount`, `gitServer.gitea.config.service.DISABLE_REGISTRATION` |
| `GIT_SERVER_SECURITY_CONTEXT_RUN_AS_USER` | `gitServer.podSecurityContext.runAsUser` |

Use native booleans, integers, maps, and arrays instead of YAML encoded in strings.
The same chart-shaped pattern applies to resource limits, scheduling,
service accounts, and security contexts. The Git server action adopts an existing
PVC and sets `gitServer.persistence.create` from the helper's boolean output.

Runtime registry addresses, storage class, injector details, and credentials come
from `.State`. Components explicitly request the credential or certificate groups
they use through `stateAccess`. The reused agent chart still uses its existing
built-ins for webhook CA bundles and the mutation policy. The injector destination
uses the existing `###ZARF_TEMP###` deployment placeholder. No legacy package
variables, constants, or `###ZARF_PKG_TMPL_*###` substitutions are used here.

The [variables-to-values guide](https://docs.zarf.dev/best-practices/variables-to-values/)
describes the migration; these definitions use the v1beta1 field names
`enableTemplating` and `valuesFiles[].path`.

## Remote components

Each service definition in its own directory is independently publishable
with `zarf component publish`. The registry and seed registry share a local base;
publishing resolves that import and includes the shared resources. References to
existing charts and manifests outside this directory are bundled and normalized
by the publisher. Components contain no `onCreate` actions.

For future publication, first render with an agent image available from a registry:

```sh
make template-init-package-v1beta1 \
  INIT_V1BETA1_TEMPLATE_SET='agent.image=ghcr.io/zarf-dev/zarf/agent:v0.87.0,agent.source=registry'
```

Then publish the agent, registry, and Git server's `zarf.gen.yaml` definitions to
your chosen OCI repository using
`zarf component publish <directory>/zarf.gen.yaml oci://<repository>` from this
directory. Also publish `registry/seed-registry.gen.yaml` for the seed registry.
For K3s and the injector, publish each directory's `amd64.gen.yaml` and
`arm64.gen.yaml` to the same repository and version to provide both variants.
Both variants are rendered on every build, and package imports select the matching
architecture. Publishing resolves `k3s/common.gen.yaml` and
`registry/registry-base.gen.yaml` into their consumers; these shared definitions
do not need separate publication. The other components use images selected for
the package architecture at creation time.

Consumers can replace a local import with:

```yaml
import:
  remote:
    - url: oci://<repository>/zarf-agent:<version>
```

Import these service components into a `ZarfInitConfig`. Preserve the init service
order and the conventional names, including `zarf-agent`, because the reused agent
chart uses component-specific built-ins. Make targets only render and create local
packages; they do not publish artifacts.
