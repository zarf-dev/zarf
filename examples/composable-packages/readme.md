<!-- FIXME: this should be published to ghcr -->
This example demonstrates component imports using small ConfigMaps. It has no container images and owns all of its component definitions and manifests.

The reusable application component imports a shared base component. The package then imports the application twice: once from the local filesystem and once from an OCI registry.

| Definition | What it demonstrates |
| --- | --- |
| `components/base.yaml` | Defines the `configuration` manifest group with a shared ConfigMap and a default namespace. |
| `components/application.yaml` | Imports the base and appends an application ConfigMap to the same manifest group. |
| `zarf.yaml`, `local-import` | Imports the application, overrides the namespace to `imports-local`, and appends a package-specific ConfigMap. |
| `zarf.yaml`, `remote-import` | Imports the published application and overrides the namespace to `imports-remote`. |

Resources with matching names merge. In this example, `manifests[].namespace` is overridden, while `manifests[].files` is appended. The local import therefore deploys three ConfigMaps; the remote import deploys the two reusable ConfigMaps.

Start a local registry, publish the application component, and create the package from the repository root:

```bash
docker run -d --rm -p 5000:5000 --name zarf-example-registry registry:3
zarf component publish examples/composable-packages/components/application.yaml oci://localhost:5000 --plain-http
zarf package create examples/composable-packages --plain-http --confirm
```

Publication resolves the application's nested local import and bundles both reusable manifests. Consumers of the published component do not need the original component files. `local-settings.yaml` belongs to the consuming package and is not published.

For another registry, change `import.remote[].url` in `zarf.yaml` and the publish destination. Use `--plain-http` only for registries serving HTTP.

With a reachable Kubernetes cluster, deploy the package and compare the results:

```bash
zarf package deploy zarf-package-composable-packages-*.tar.zst --connected --confirm
kubectl get configmaps -n imports-local
kubectl get configmaps -n imports-remote
```

`--connected` allows this example to deploy without initializing Zarf on the cluster. The ConfigMaps contain no namespace, so each consuming component controls where they are deployed.

The `import.local` and `import.remote` fields are lists to support alternative component variants. Zarf selects exactly one matching source by flavor and architecture at package creation. Listing multiple sources does not combine them; nested imports are how this example builds on another component.

Remove the package and stop the example registry when finished:

```bash
zarf package remove composable-packages --confirm
docker stop zarf-example-registry
```
