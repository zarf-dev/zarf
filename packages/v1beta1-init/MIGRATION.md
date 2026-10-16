# Switching from init variables to values

When deploying the v1beta1 init package, replace legacy `--set` overrides and
`ZARF_VAR_*` environment variables with `--values` or `--set-values`. Variable names
are not translated automatically. Settings you omit use the package defaults.

## Variable replacements

All 49 declared deploy-time variables from the v1alpha1 init components map as follows:

| v1alpha1 variable | v1beta1 value |
| --- | --- |
| `K3S_ARGS` | `k3s.args` |
| `AGENT_AFFINITY` | `agent.affinity` |
| `AGENT_TOLERATIONS` | `agent.tolerations` |
| `AGENT_NODE_SELECTOR` | `agent.nodeSelector` |
| `AGENT_WEBHOOK_ANNOTATIONS` | `agent.webhookAnnotations` |
| `AGENT_MUTATION_EXCLUSIONS` | `agent.mutationExclusions` |
| `REGISTRY_EXISTING_PVC` | `registry.persistence.existingClaim` |
| `REGISTRY_PVC_ENABLED` | `registry.persistence.enabled` |
| `REGISTRY_PVC_SIZE` | `registry.persistence.size` |
| `REGISTRY_PVC_ACCESS_MODE` | `registry.persistence.accessMode` |
| `REGISTRY_CPU_REQ` | `registry.resources.requests.cpu` |
| `REGISTRY_MEM_REQ` | `registry.resources.requests.memory` |
| `REGISTRY_CPU_LIMIT` | `registry.resources.limits.cpu` |
| `REGISTRY_MEM_LIMIT` | `registry.resources.limits.memory` |
| `REGISTRY_HPA_MIN` | `registry.autoscaling.minReplicas` |
| `REGISTRY_HPA_MAX` | `registry.autoscaling.maxReplicas` |
| `REGISTRY_HPA_ENABLE` | `registry.autoscaling.enabled` |
| `REGISTRY_CA_BUNDLE` | `registry.caBundle` |
| `REGISTRY_EXTRA_ENVS` | `registry.extraEnvVars` |
| `REGISTRY_OTEL_EXPORTER` | `registry.opentelemetry.exporter` |
| `REGISTRY_CREATE_SERVICE_ACCOUNT` | `registry.serviceAccount.create` |
| `REGISTRY_SERVICE_ACCOUNT_NAME` | `registry.serviceAccount.name` |
| `REGISTRY_SERVICE_ACCOUNT_ANNOTATIONS` | `registry.serviceAccount.annotations` |
| `REGISTRY_AFFINITY_ENABLE` | `registry.affinity.enabled` |
| `REGISTRY_AFFINITY_CUSTOM` | `registry.affinity.custom` |
| `REGISTRY_TOLERATIONS` | `registry.tolerations` |
| `REGISTRY_NODE_SELECTOR` | `registry.nodeSelector` |
| `PROXY_TOLERATIONS` | `registry.proxy.tolerations` |
| `REGISTRY_HPA_AUTO_SIZE` | `registry.autoscaling.mapReplicasToNodes` |
| `REGISTRY_HPA_TARGET_CPU` | `registry.autoscaling.targetCPUUtilizationPercentage` |
| `HOST_NETWORK_PROXY` | `registry.proxy.hostNetwork` |
| `REGISTRY_SECURITY_CONTEXT_RUN_AS_USER` | `registry.securityContext.runAsUser` |
| `REGISTRY_SECURITY_CONTEXT_FS_GROUP` | `registry.securityContext.fsGroup` |
| `REGISTRY_SECURITY_CONTEXT_RUN_AS_GROUP` | `registry.securityContext.runAsGroup` |
| `GIT_SERVER_EXISTING_PVC` | `gitServer.persistence.claimName` |
| `GIT_SERVER_PVC_SIZE` | `gitServer.persistence.size` |
| `GIT_SERVER_PVC_ACCESS_MODE` | `gitServer.persistence.accessModes` (array) |
| `GIT_SERVER_CPU_REQ` | `gitServer.resources.requests.cpu` |
| `GIT_SERVER_MEM_REQ` | `gitServer.resources.requests.memory` |
| `GIT_SERVER_CPU_LIMIT` | `gitServer.resources.limits.cpu` |
| `GIT_SERVER_MEM_LIMIT` | `gitServer.resources.limits.memory` |
| `GIT_SERVER_REPLICA_COUNT` | `gitServer.replicaCount` |
| `GIT_SERVER_DISABLE_REGISTRATION` | `gitServer.gitea.config.service.DISABLE_REGISTRATION` |
| `GIT_SERVER_AFFINITY` | `gitServer.affinity` |
| `GIT_SERVER_TOLERATIONS` | `gitServer.tolerations` |
| `GIT_SERVER_NODE_SELECTOR` | `gitServer.nodeSelector` |
| `GIT_SERVER_SECURITY_CONTEXT_RUN_AS_USER` | `gitServer.podSecurityContext.runAsUser` |
| `GIT_SERVER_SECURITY_CONTEXT_FS_GROUP` | `gitServer.podSecurityContext.fsGroup` |
| `GIT_SERVER_SECURITY_CONTEXT_RUN_AS_GROUP` | `gitServer.podSecurityContext.runAsGroup` |

Write maps, arrays, booleans, and numbers as native YAML rather than strings
containing YAML. For example, save this as `init-values.yaml`:

```yaml
registry:
  persistence:
    size: 40Gi
  proxy:
    hostNetwork: true
agent:
  nodeSelector:
    role: infra
gitServer:
  persistence:
    accessModes:
      - ReadWriteOnce
```

Deploy with your v1beta1 init package filename:

```sh
zarf init ./zarf-init-amd64-VERSION.tar.zst --confirm \
  --features=values=true --values init-values.yaml
```

For a single setting, use `--set-values registry.persistence.size=40Gi` instead.
`--set-values` takes precedence over values files. Select optional components with
`--components` as before; setting their values does not enable them.

For `REGISTRY_CA_BUNDLE`, put the certificate contents in `registry.caBundle`, not
a file path. Packaged images are selected when the package is built, and runtime
addresses and credentials are supplied by Zarf; neither needs a deploy-time image
or credential variable.

`GIT_SERVER_CREATE_PVC` was set by a deployment action, not supplied by deployers.
Its replacement, `gitServer.persistence.create`, is still set automatically.
