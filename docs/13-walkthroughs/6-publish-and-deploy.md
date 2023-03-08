# Publishing + Deploying A Zarf Package Via OCI

## Introduction

In this walkthrough, we are going to run through how to publish a Zarf package to an [OCI](https://github.com/opencontainers/image-spec) compliant registry, allowing end users to pull and deploy packages without needing to build locally, or transfer the package to their environment.

![Walkthrough GIF](../.images/walkthrough-6.gif)

## Prerequisites

For following along locally, please ensure the following prerequisites are met:

1. Zarf binary installed on your `$PATH`: ([Install Instructions](../3-getting-started.md#installing-zarf))
2. Access to a [Registry supporting the OCI Distribution Spec](https://oras.land/implementors/#registries-supporting-oci-artifacts), this walkthrough will be using Docker Hub
3. The following operations:

```bash
# Setup some variables for the registry we will be using
$ REGISTRY=docker.io
$ set +o history
$ REGISTRY_USERNAME=<username> # <-- replace with your username
$ REPOSITORY_URL=$REGISTRY/$REGISTRY_USERNAME
$ REGISTRY_SECRET=<secret> # <-- replace with your password or auth token
$ set -o history

# Authenticate with your registry using Zarf
$ echo $REGISTRY_SECRET | zarf tools registry login $REGISTRY --username $REGISTRY_USERNAME --password-stdin
# (Optional) Otherwise, create a Docker compliant auth config file if the Docker CLI is not installed
$ mkdir -p ~/.docker
$ AUTH=$(echo -n "$REGISTRY_USERNAME:$REGISTRY_SECRET" | base64)
# Note: If using Docker Hub, the registry URL is `https://index.docker.io/v1/` for the auth config
$ cat <<EOF > ~/.docker/config.json
{
  "auths": {
    "$REGISTRY": {
      "auth": "$AUTH"
    }
  }
}
EOF
```

## Publish Package

First, create a valid Zarf package definition (`zarf.yaml`), with the `metadata.version` key set.

```yaml
# Make a new directory to work in
$ mkdir -p zarf-publish-walkthrough && cd zarf-publish-walkthrough

# For this walkthrough we will use the `helm-oci-chart` example package
# located here: https://github.com/defenseunicorns/zarf/blob/main/examples/helm-oci-chart/zarf.yaml
$ cat <<EOF > zarf.yaml
kind: ZarfPackageConfig
metadata:
  name: helm-oci-chart
  description: Deploy podinfo using a Helm OCI chart
  # Note: In order to publish, the package must have a version
  version: 0.0.1

components:
  - name: helm-oci-chart
    required: true
    charts:
      - name: podinfo
        version: 6.3.3
        namespace: helm-oci-demo
        url: oci://ghcr.io/stefanprodan/charts/podinfo
    images:
      - "ghcr.io/stefanprodan/podinfo:6.3.3"
EOF
```

Create the package locally:

```bash
# Create the package (interactively)
$ zarf package create .
# Make these choices at the prompts:
# Create this Zarf package? Yes
# Please provide a value for "Maximum Package Size" 0
```

Then publish the package to the registry:

```bash
# Your package tarball may be named differently based on your machine's architecture
$ zarf package publish zarf-package-helm-oci-chart-arm64-0.0.1.tar.zst oci://$REPOSITORY_URL

...

  •  Publishing package to $REPOSITORY_URL/helm-oci-chart:0.0.1-arm64
  ✔  Prepared 14 layers
  ✔  515aceaacb8d images/index.json
  ✔  4615b4f0c1ed zarf.yaml
  ✔  1300d6545c84 sboms.tar
  ✔  b66dbb27a733 images/oci-layout
  ✔  46564f0eff85 images/blobs/sha256/46564f0...06008f762391a7bb7d58f339ee
  ✔  4f4fb700ef54 images/blobs/sha256/4f4fb70...b5577484a6d75e68dc38e8acc1
  ✔  6ff8f4799d50 images/blobs/sha256/6ff8f47...4bc00ec8b988d28cef78ea9a5b
  ✔  74eae207aa23 images/blobs/sha256/74eae20...fcb007d3da7b842313f80d2c33
  ✔  a9eaa45ef418 images/blobs/sha256/a9eaa45...6789c52a87ba5a9e6483f2b74f
  ✔  8c5b695f4724 images/blobs/sha256/8c5b695...014f94c8d4ea62772c477c1e03
  ✔  ab67ffd6e92e images/blobs/sha256/ab67ffd...f8c9d93c0e719f6350e99d3aea
  ✔  b95c82728c36 images/blobs/sha256/b95c827...042a9c5d84426c1674044916d4
  ✔  e2b45cdcd8bf images/blobs/sha256/e2b45cd...000f1bc1695014e38821dc675c
  ✔  79be488a834e components/helm-oci-chart.tar
  ✔  d8399f7b56ca [application/vnd.unknown.config.v1+json]
  ✔  aed84ba183e7 [application/vnd.oci.image.manifest.v1+json]
  ✔  Published $REPOSITORY_URL/helm-oci-chart:0.0.1-arm64 [application/vnd.oci.image.manifest.v1+json]

  •  To inspect/deploy/pull:
  •  zarf package inspect oci://$REPOSITORY_URL/helm-oci-chart:0.0.1-arm64 --insecure
  •  zarf package deploy oci://$REPOSITORY_URL/helm-oci-chart:0.0.1-arm64 --insecure
  •  zarf package pull oci://$REPOSITORY_URL/helm-oci-chart:0.0.1-arm64 --insecure
```

:::note

The name and reference of this OCI artifact is derived from the package metadata, e.g.: `helm-oci-chart:0.0.1-arm64`

To modify, edit `zarf.yaml` and re-run `zarf package create .`

:::

## Inspect Package

Inspecting a Zarf package stored in an OCI registry is the same as inspecting a local package and has the same flags:

```yaml
$ zarf package inspect oci://$REPOSITORY_URLhelm-oci-chart:0.0.1-arm64
---
kind: ZarfPackageConfig
metadata:
  name: helm-oci-chart
  description: Deploy podinfo using a Helm OCI chart
  version: 0.0.1
  architecture: arm64
build:
  terminal: minimind.local
  user: whoami
  architecture: arm64
  timestamp: Tue, 07 Mar 2023 14:27:25 -0600
  version: v0.25.0-rc1-41-g07d61ba7
  migrations:
    - scripts-to-actions
components:
  - name: helm-oci-chart
    required: true
    charts:
      - name: podinfo
        url: oci://ghcr.io/stefanprodan/charts/podinfo
        version: 6.3.3
        namespace: helm-oci-demo
    images:
      - ghcr.io/stefanprodan/podinfo:6.3.3
```

## Deploy Package

Deploying a package stored in an OCI registry is nearly the same experience as deploying a local package:

```bash
# Due to the length of the console output from this command, it has been omitted from this walkthrough
$ zarf package deploy oci://$REPOSITORY_URL/helm-oci-chart:0.0.1-arm64
# Make these choices at the prompts:
# Deploy this Zarf package? Yes

$ zarf packages list

    Package        | Components
    helm-oci-chart | [helm-oci-chart]
    init           | [zarf-injector zarf-seed-registry zarf-registry zarf-agent git-server]
```
