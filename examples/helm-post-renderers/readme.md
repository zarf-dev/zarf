This example uses [Helm chart post-renderers](/ref/components/#post-renderers) to fix a gap in the upstream [Velero chart](https://github.com/vmware-tanzu/helm-charts/tree/cd8980cd6dbf002d63b1da6860cd8813a3764dbb/charts/velero) (`12.2.5`) that its values can't reach. Velero is a backup tool that many airgapped clusters run. Post-renderers require a `zarf.dev/v1beta1` package and Zarf `v1.1.0` or later.

## The Problem

Hardened clusters often enforce the [Pod Security "restricted" standard](https://kubernetes.io/docs/concepts/security/pod-security-standards/#restricted). They might use namespace labels, a cluster-wide admission default such as the RKE2 CIS profile, or a policy engine. Under that standard, the API server rejects any pod that does not set `runAsNonRoot`, `seccompProfile`, `allowPrivilegeEscalation: false`, and `capabilities.drop: ["ALL"]`.

Most of the Velero chart can comply through values. `podSecurityContext` and `containerSecurityContext` cover the Velero Deployment and the `velero-upgrade-crds` hook Job. `nodeSelector` and `tolerations` cover their scheduling. `velero-values.yaml` sets those values.

The chart can also label its own namespace. Its values file [suggests using `namespace.labels`](https://github.com/vmware-tanzu/helm-charts/blob/cd8980cd6dbf002d63b1da6860cd8813a3764dbb/charts/velero/values.yaml#L5-L12) for Pod Security labels. Those labels are applied by [`velero-label-namespace`](https://github.com/vmware-tanzu/helm-charts/blob/cd8980cd6dbf002d63b1da6860cd8813a3764dbb/charts/velero/templates/label-namespace/labelnamespace.yaml), a Helm hook Job that runs after every install, upgrade, and rollback. In this chart version, that Job:

- has no pod or container `securityContext`, and no values to set one
- runs `registry.k8s.io/kubectl`, which runs as root by default
- ignores the chart's `nodeSelector` and `tolerations` values

On the first install, the namespace isn't restricted yet, so everything deploys. On the next `zarf package deploy`, which runs a Helm upgrade, the namespace enforces "restricted". The API server rejects the Job's pod and the upgrade fails at the post-upgrade hook:

```text
Error creating: pods "velero-label-namespace-..." is forbidden: violates PodSecurity "restricted:latest":
allowPrivilegeEscalation != false (container "label-namespace" must set securityContext.allowPrivilegeEscalation=false),
unrestricted capabilities (container "label-namespace" must set securityContext.capabilities.drop=["ALL"]),
runAsNonRoot != true (pod or container "label-namespace" must set securityContext.runAsNonRoot=true),
seccompProfile (pod or container "label-namespace" must set securityContext.seccompProfile.type to "RuntimeDefault" or "Localhost")
```

If every node in the cluster is tainted, for example in a dedicated backup node pool, the same Job also stays `Pending` because it cannot get tolerations.

Changing values can't fix this; the alternatives are forking the chart or relaxing cluster policy. This is the gap [post-renderers](https://fluxcd.io/flux/components/helm/helmreleases/#post-renderers) were made for. Many charts have similar gaps in hook Jobs, which Helm 3 post-renderers could not patch at all.

## The Fix

The chart entry in `zarf.yaml` keeps using values for everything the chart exposes. It adds two Kustomize post-renderers that target only the hook Job:

1. A static strategic-merge patch that adds a restricted-compliant pod and container `securityContext`.
2. A patch with `enableTemplating: true` that copies the `scheduling.nodeSelector` and `scheduling.tolerations` package values into the Job. These are the same values the chart receives through `charts[].values`, so one setting schedules every Velero pod.

Because the target is a Helm hook, the chart sets `postRenderStrategy: separate`. The default strategy, `nohooks`, never passes hooks to post-renderers. `combined` would also work for this chart, because no hook shares a kind, name, and namespace with a regular resource. `separate` keeps the hook pass independent of the chart's regular resources.

## Try It Out

Review the package's `zarf.yaml`, where the post-renderers are set on the `velero` chart. Then inspect the post-rendered manifests before building:

```bash
zarf dev inspect manifests examples/helm-post-renderers

# See the scheduling patch with deploy-time values for tainted nodes
zarf dev inspect manifests examples/helm-post-renderers --values examples/helm-post-renderers/tainted-nodes-values.yaml
```

Create and deploy the package, then deploy it again to run a Helm upgrade into the now-restricted namespace:

```bash
zarf package create examples/helm-post-renderers --confirm
zarf package deploy zarf-package-helm-post-renderers-*.tar.zst --confirm
zarf package deploy zarf-package-helm-post-renderers-*.tar.zst --confirm
```

To schedule Velero on dedicated nodes, pass your own values file. Lists such as `tolerations` can't be set with `--set-values`.

```bash
zarf package deploy zarf-package-helm-post-renderers-*.tar.zst --values examples/helm-post-renderers/tainted-nodes-values.yaml --confirm
```

## Verify

```bash
# The namespace enforces "restricted" (labels applied by the hook Job)
zarf tools kubectl get namespace velero --show-labels

# The stored release hooks include the patched securityContext, nodeSelector, and tolerations
zarf tools helm get hooks velero -n velero

# The built package shows the same post-rendered Job
zarf package inspect manifests zarf-package-helm-post-renderers-*.tar.zst
```

:::note

Images that a post-renderer adds or changes must be listed in the component's `images`. This example doesn't change any images. The `kubectl` tag is pinned in `velero-values.yaml` because, by default, the chart derives it from the cluster's Kubernetes version at deploy time, so it can't be known when the package is created.

:::
