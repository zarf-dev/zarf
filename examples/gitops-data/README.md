# Zarf Simple GitOps Service Update

This examples shows how to package images and repos to be loaded into the
GitOps service.  This package does not deploy anything itself, but pushes
assets to the GitOps service to be consumed by the GitOps engine of your
choice.

## Demonstrated Features

### Docker Image Push

This example demonstrates using component `images` to push container images
to an image registry. Images provided to the `images` tag are
uploaded to a Zarf hosted docker registry, which can be later used by
Kubernetes manifests, or manually used as shown in this guide.

### Tag-Provided Git Repository Clone

Tag-provided git repository cloning is the recommended way of cloning a git
repository for air-gapped deployment. Tag-provided clones are defined using
the `url.git@tag` format as seen in the example with the `defenseunicorns/zarf`
repository (`https://github.com/defenseunicorns/zarf.git@v0.12.0`).

A tag-provided clone only mirrors the tag defined in the Zarf definition. The
tag will appear on the Gitea mirror as the default branch name of the
repository being mirrored, and the tag itself.

### Git Repository Full Clone

Full clones are used in this example by the `stefanprodan/podinfo` repository,
following the `url.git` format (`https://github.com/stefanprodan/podinfo.git`).
Full clones will contain **all** branches and tags in the mirrored repository
rather than any one specific tag.

## Prerequisites

This example assumes you have already created a Zarf cluster. If that is not
the case, refer to the below locations in the game example README. Be sure when
creating the Zarf cluster to deploy the GitOps component!

1. [Prepare the Zarf Environment](../game/README.md#get-ready)
1. [Create a Zarf Cluster](../game/README.md#create-a-cluster)

## Create the Zarf Package

To create this Zarf package run the below command:

```sh
cd <zarf dir>/examples/gitops-data # directory with zarf.yaml
zarf package create                # make the package
```

Successful execution will create a package named
`zarf-package-gitops-service-data-<arch>.tar.zst`, the Zarf example package.

## Deploying the Zarf Package

To deploy the Zarf package, copy it to a machine that has a Zarf cluster
deployed with the GitOps component enabled and the `zarf` executable accessible
in your `PATH`.

With the Zarf package in the current working directory, execute the below
command to deploy the package, uploading the Git repositories to Gitea and the
container images to the Docker registry.

```sh
zarf package deploy zarf-package-gitops-service-data-<arch>.tar.zst
```

## Applying the Kustomization

Once the package has been deployed, the Kustomization can be applied from the
Gitea repository using the below command.

```sh
# Run 'zarf connect' and send it to the background
zarf connect git&

# Apply the kustomization
kubectl apply -k http://zarf-git-user:$(zarf tools get-admin-password)@localhost:<WhicheverPortGotUsed>/zarf-git-user/mirror__github.com__stefanprodan__podinfo//kustomize

# Inspect
zarf tools k9s

# Bring the connection back to the foreground
fg

# Kill the connection with Ctrl-C
```

## Clean Up

Clean up simply by just deleting the whole cluster

```sh
kind delete cluster
```
