# Zarf Nerd Notes 

:::caution Hard Hat Area
This page is still being developed. More content will be added soon!
:::


Zarf is written entirely in [go](https://go.dev/), except for a single 400Kb binary for the injector system written in [rust](https://www.rust-lang.org/), so we can fit it in a [configmap](https://kubernetes.io/docs/concepts/configuration/configmap/). All assets are bundled together into a single [zstd](https://facebook.github.io/zstd/) tarball on each `zarf package create` operation. On the air gap / offline side, `zarf package deploy` extracts the various assets and places them on the filesystem or installs them in the cluster, depending on what the zarf package says to do. Some important ideas behind Zarf:

- All workloads are installed in the cluster via the [Helm SDK](https://helm.sh/docs/topics/advanced/#go-sdk)
- The OCI Registries used are both from [Docker](https://github.com/distribution/distribution)
- Currently, the Registry and Git servers _are not HA_, see [#375](https://github.com/defenseunicorns/zarf/issues/376) and [#376](https://github.com/defenseunicorns/zarf/issues/376) for discussion on this
- To avoid TLS issues, Zarf binds to `127.0.0.1:31999` on each node as a [NodePort](https://kubernetes.io/docs/concepts/services-networking/service/#type-nodeport) to allow all nodes to access the pod(s) in the cluster
- Until [#306](https://github.com/defenseunicorns/zarf/pull/306) is merged, during helm install/upgrade a [Helm PostRender](https://helm.sh/docs/topics/advanced/#post-rendering) function is called to mutate images and [ImagePullSecrets](https://kubernetes.io/docs/concepts/containers/images/#specifying-imagepullsecrets-on-a-pod) so the deployed resources use the NodePort binding
- Zarf uses a custom injector system to bootstrap a new cluster. See the PR [#329](https://github.com/defenseunicorns/zarf/pull/329) and [ADR](https://github.com/defenseunicorns/zarf/blob/master/docs/adr/0003-image-injection-into-remote-clusters-without-native-support.md) for more details on how we came to this solution.  The general steps are listed below:
  - Get a list of images in the cluster
  - Attempt to create an ephemeral pod using an image from the list
  - A small rust binary that is compiled using [musl](https://www.musl-libc.org/) to keep the size the max binary size of ~ 672 KBs is injected into the pod
  - The mini zarf registry binary and `docker:2` images are put in a tar archive and split into 512 KB chunks; larger sizes tended to cause latency issues on low-resource control planes
  - An init container runs the rust binary to re-assemble and extract the zarf binary and registry image
  - The container then starts and runs the zarf binary to host the registry image in an embedded docker registry
  - After this, the main docker registry chart is deployed, pulls the image from the ephemeral pod, and finally destroys the created configmaps, pod, and service

<br />

<!-- TODO: @JPERRY I think this svg is out of date now that the 'Zarf agent' PR got merged.. -->
### Zarf Architecture
![Architecture Diagram](../.images/architecture.drawio.svg)
