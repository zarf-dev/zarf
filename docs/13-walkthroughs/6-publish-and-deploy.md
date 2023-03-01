# Publishing + Deploying A Zarf Package Via OCI

## Introduction

In this walkthrough, we are going to run through how to publish a Zarf package to an [OCI](https://github.com/opencontainers/image-spec) compliant registry, allowing end users to pull and deploy packages without needing to build locally, or transfer the package to their environment.

![Walkthrough GIF](../.images/walkthrough-6.gif)

## Prerequisites

For following along locally, please ensure the following prerequisites are met:

1. Zarf binary installed on your $PATH: ([Install Instructions](../3-getting-started.md#installing-zarf))
2. Access to a [Registry supporting the OCI Distribution Spec](https://oras.land/implementors/#registries-supporting-oci-artifacts), this walkthrough will be using Docker Hub
3. The following operations:

```bash
# Setup some variables for the registry we will be using
$ REGISTRY=docker.io
$ set +o history
$ REGISTRY_USERNAME=<username> # <-- replace with your username
$ REGISTRY_SECRET=<secret> # <-- replace with your password or auth token
$ set -o history

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

## Publishing A Zarf Package

First create the package locally:

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
$ zarf package publish zarf-package-helm-oci-chart-arm64.tar.zst oci://$REGISTRY/$USERNAME
```

:::note

The name and reference of this OCI artifact is derived from the package metadata, e.g.: `helm-oci-chart:0.0.1-arm64`

To modify, edit `zarf.yaml` and re-run `zarf package create .`

:::

## Inspecting a Published Zarf Package

```yaml
$ zarf package inspect oci://$REGISTRY_URL/$USERNAME/helm-oci-chart

tags:
- 0.0.1-arm64
latest:
  tag: 0.0.1-arm64
  descriptor:
    mediaType: application/vnd.oci.artifact.manifest.v1+json
    digest: sha256:624e97e9b235ebd8eab699238482eebd2f535683755323eb40c819e0efdcd959
    size: 3338
```

## Deploying A Zarf Package From The Registry

```bash
$ zarf package deploy oci://$REGISTRY/$USERNAME/helm-oci-chart:0.0.1-arm64
# zarf package deploy oci://REGISTRY/NAMESPACE/NAME:VERSION
# zarf package deploy oci://docker.io/defenseunicorns/strimzi:v0.24.0-arm64
```
