# FAQ

## Do I have to use Homebrew to install Zarf?

No, the Zarf binary and init package can be downloaded from the [Releases Page](https://github.com/defenseunicorns/zarf/releases). Zarf does not need to be installed or available to all users on the system, but it does need to be executable for the current user (i.e. `chmod +x zarf` for Linux/Mac).

## What dependencies does Zarf have?

Zarf is statically compiled and written in [Go](https://golang.org/) and [Rust](https://www.rust-lang.org/), so it has no external dependencies. For Linux, Zarf can bring a Kubernetes cluster using [K3s](https://k3s.io/). For Mac and Windows, Zarf can leverage any available local or remote cluster the user has access to. Currently, the K3s installation Zarf performs does require a [Systemd](https://en.wikipedia.org/wiki/Systemd) based system and `root` (not just `sudo`) access.

## What license is Zarf under?

Zarf is under the [Apache License 2.0](https://github.com/defenseunicorns/zarf/blob/main/LICENSE). This is one of the most commonly used licenses for open source software.

## What is the Zarf Agent?

The Zarf Agent is a [Kubernetes Mutating Webhook](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#mutatingadmissionwebhook) that is installed into the cluster during `zarf init`. The Agent is responsible for modifying [Kubernetes PodSpec](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/#PodSpec) objects [Image](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/#Container.Image) fields to point to the Zarf Registry. This allows the cluster to pull images from the Zarf Registry instead of the internet without having to modify the original image references. The Agent also modifies [Flux GitRepository](https://fluxcd.io/docs/components/source/gitrepositories/) objects to point to the local Git Server.

## Why doesn't the Zarf Agent create secrets it needs in the cluster?

During early discussions and [subsequent decision](../adr/0005-mutating-webhook.md) to use a Mutating Webhook, we decided to not have the Agent create any secrets in the cluster. This is to avoid the Agent having to have more privileges than it needs as well as to avoid collisions with Helm. The Agent today simply responds to requests to patch PodSpec and GitRepository objects.

The Agent does not need to create any secrets in the cluster. Instead, during `zarf init` and `zarf package deploy`, secrets are automatically created as [Helm Postrender Hook](https://helm.sh/docs/topics/advanced/#post-rendering) for any namespaces Zarf sees. If you have resources managed by [Flux](https://fluxcd.io/) that are not in a namespace managed by Zarf, you can either create the secrets manually or include a manifest to create the namespace in your package and let Zarf create the secrets for you.

## How can a Kubernetes resource be excluded from the Zarf Agent?

Resources can be excluded at the namespace or resources level by adding the `zarf.dev/agent: ignore` label.

## What happens to resources that exist in the cluster before `zarf init`?

During the `zarf init` operation, the Zarf Agent will patch any existing namespaces with the `zarf.dev/agent: ignore` label to prevent the Agent from modifying any resources in that namespace. This is done because there is no way to guarantee the images used by pods in existing namespaces are available in the Zarf Registry.

## How can I improve the speed of loading large images from Docker on `zarf package create`?

Due to some limitations with how Docker provides access to local image layers, `zarf package create` has to rely on `docker save` under the hood which is [very slow overall](https://github.com/defenseunicorns/zarf/issues/1214) and also takes a long time to report progress. We experimented with many ways to improve this, but for now recommend leveraging a local docker registry to speed up the process. This can be done by running a local registry and pushing the images to it before running `zarf package create`. This will allow `zarf package create` to pull the images from the local registry instead of Docker. This can also be combined with [component actions](4-user-guide/5-component-actions.md) to make the process automatic. Given an example image of `my-giant-image:###ZARF_PKG_VAR_IMG###` you could do something like this:

```sh
# Create a local registry
docker run -d -p 5000:5000 --restart=always --name registry registry:2

# Run the package create with a tag variable
zarf package create --set IMG=my-giant-image:v2
```

```yaml
kind: ZarfPackageConfig
metadata:
  name: giant-image-example

components:
  - name: main
    actions:
      # runs during "zarf package create"
      onCreate:
        # runs before the component is created
        before:
          - cmd: 'docker tag ###ZARF_PKG_VAR_IMG### localhost:5000/###ZARF_PKG_VAR_IMG###'
          - cmd: 'docker push localhost:5000/###ZARF_PKG_VAR_IMG###'

    images:
      - 'localhost:5000/###ZARF_PKG_VAR_IMG###'
```

## Can I pull in more than http(s) git repos on `zarf package create`?

Under the hood, Zarf uses [`go-git`](https://github.com/go-git/go-git) to perform `git` operations, but it can fallback to `git` located on the host and thus supports any of the [git protocols](https://git-scm.com/book/en/v2/Git-on-the-Server-The-Protocols) available.  All you need to use a different protocol is to specify the full URL for that particular repo:

:::note

In order for the fallback to work correctly you must have `git` version `2.14` or later in your path.

:::

```yaml
kind: ZarfPackageConfig
metadata:
  name: repo-schemes-example

components:
    repos:
      - https://github.com/defenseunicorns/zarf.git
      - ssh://git@github.com/defenseunicorns/zarf.git
      - file:///home/zarf/workspace/zarf
      - git://somegithost.com/zarf.git
```

In the airgap, Zarf with rewrite these URLs to match the scheme and host of the provided airgap `git` server.

:::note

When specifying other schemes in Zarf you must change the consuming side as well since Zarf will add a CRC hash of the URL to the repo name on the airgap side.  This is to reduce the chance for collisions between repos with similar names.  This means an example Flux `GitRepository` specification would look like this for the `file://` based pull:

```yaml
---
apiVersion: source.toolkit.fluxcd.io/v1beta2
kind: GitRepository
metadata:
  name: podinfo
  namespace: flux-system
spec:
  interval: 30s
  ref:
    tag: 6.1.6
  url: file:///home/zarf/workspace/podinfo
```

:::

## What is YOLO Mode and why would I use it?

YOLO Mode is a special package metadata designation that be added to a package prior to `zarf package create` to allow the package to be installed without the need for a `zarf init` operation. In most cases this will not be used, but it can be useful for testing or for environments that manage their own registries and Git servers completely outside of Zarf. This can also be used as a way to transition slowly to using Zarf without having to do a full migration.

:::note
Typically you should not deploy a Zarf package in YOLO mode if the cluster has already been initialized with Zarf. This could lead to an [ImagePullBackOff](https://kubernetes.io/docs/concepts/containers/images/#imagepullbackoff) if the resources in the package do not include the `zarf.dev/agent: ignore` label and are not already available in the Zarf Registry.
:::
