# Git Data

This examples shows how to package `git` repositories to be bundled and pushed across the airgap.  This package does not deploy anything itself, but pushes assets to the specified `git` service to be consumed as desired.  Within Zarf, their are tow main ways to include `git` repositories as described below.

:::info

To view the example source code, select the `Edit this page` link below the article and select the parent folder.

:::

&nbsp;


## Tag-Provided Git Repository Clone

Tag-provided `git` repository cloning is the recommended way of cloning a `git` repository for air-gapped deployments because it wraps meaning around a specific point in git history that can easily be traced back to the online world. Tag-provided clones are defined using the `url.git@tag` format as seen in the example of the `defenseunicorns/zarf` repository (`https://github.com/defenseunicorns/zarf.git@v0.15.0`).

A tag-provided clone only mirrors the tag defined in the Zarf definition. The tag will be applied on the `git` mirror to the default trunk branch of the repo (i.e. `master`, `main`, or the default when the repo is cloned).

## SHA-Provided Git Repository Clone

SHA-provided `git` repository cloning is another supported way of cloning repos in Zarf but is not recommended as it is less readable / understandable than tag cloning.  Commit SHAs are defined using the same `url.git@sha` format as seen in the example of the `defenseunicorns/zarf` repository (`https://github.com/defenseunicorns/zarf.git@c74e2e9626da0400e0a41e78319b3054c53a5d4e`).

A SHA-provided clone only mirrors the SHA hash defined in the Zarf definition. The SHA will be applied on the `git` mirror to the default trunk branch of the repo (i.e. `master`, `main`, or the default when the repo is cloned) as Zarf does with tagging.

:::note

If you use a SHA hash or a tag that is on a separate branch this will be placed on the default trunk branch on the offline mirror (i.e. `master`, `main`, etc).  This may result in conflicts upon updates if this SHA or tag is not in the update branch's history.

:::

## Git Repository Full Clone

Full clones are used in this example with the `stefanprodan/podinfo` repository and follow the `url.git` format (`https://github.com/stefanprodan/podinfo.git`). Full clones will contain **all** branches and tags in the mirrored repository rather than any one specific tag.

&nbsp;

## Example Usage

This example assumes you have already initialized a Zarf cluster. If that is not the case, refer to the [Initializing the Cluster Walkthrough](../../docs/13-walkthroughs/1-initializing-a-k8s-cluster.md). Be sure when initializing the Zarf cluster to deploy the `git` component, or be ready to specify an external `git` repository.

### Create the Zarf Package

To create this Zarf package run the below command:

```sh
cd <zarf dir>/examples/git-data    # directory with zarf.yaml
zarf package create                # make the package
```

Successful execution will create a package named `zarf-package-git-data-<arch>.tar.zst`.

### Deploying the Zarf Package

To deploy the Zarf package, copy it to a machine that either has a Zarf cluster deployed with the `git` component or an accessible external repository and the `zarf` executable in your `PATH`.

With the Zarf package in the current working directory, execute the below command to deploy the package, uploading the Git repositories to Gitea and the container images to the Docker registry.

```sh
zarf package deploy zarf-package-git-data-<arch>.tar.zst
```

:::note

If you are using an external `git` repository you should specify it here with the git url and user flags.

:::

### Applying the Kustomization

Once the package has been deployed, the Kustomization can be applied from the `git` repository using the below command.

:::note

The following assumes you are using the internal Gitea server. If you are using an external server `zarf connect` is not required and you must change the user/url information as needed.*

:::

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
