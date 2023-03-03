# Git Data

This example shows how to package `git` repositories within a Zarf package.  This package does not deploy anything itself but pushes assets to the specified `git` service to be consumed as desired.  Within Zarf, there are a few ways to include `git` repositories (as described below).

:::info

To view the example source code, select the `Edit this page` link below the article and select the parent folder.

:::

## Tag-Based Git Repository Clone

Tag-based `git` repository cloning is the **recommended** way of cloning a `git` repository for air-gapped deployments because it wraps meaning around a specific point in git history that can easily be traced back to the online world. Tag-based clones are defined using the `scheme://host/repo@tag` format as seen in the example of the `defenseunicorns/zarf` repository (`https://github.com/defenseunicorns/zarf.git@v0.15.0`).

A tag-based clone only mirrors the tag defined in the Zarf definition. The tag will be applied on the `git` mirror to a zarf-specific branch name based on the tag name (e.g. the tag `v0.1.0` will be pushed to the `zarf-ref-v0.1.0` branch).  This ensures that this tag will be pushed and received properly by the airgap `git` server.

:::note

If you would like to use a protocol scheme other than http/https, you can do so with something like the following: `ssh://git@github.com/defenseunicorns/zarf.git@v0.15.0`.  Using this you can also clone from a local repo to help you manage larger git repositories: `file:///home/zarf/workspace/zarf@v0.15.0`.

:::

## SHA-Based Git Repository Clone

In addition to tags, Zarf also supports cloning and pushing a specific SHA hash from a `git` repository, but this is **not recommended** as it is less readable/understandable than tag cloning.  Commit SHAs are defined using the same `scheme://host/repo@shasum` format as seen in the example of the `defenseunicorns/zarf` repository (`https://github.com/defenseunicorns/zarf.git@c74e2e9626da0400e0a41e78319b3054c53a5d4e`).

A SHA-based clone only mirrors the SHA hash defined in the Zarf definition. The SHA will be applied on the `git` mirror to a zarf-specific branch name based on the SHA hash (e.g. the SHA `c74e2e9626da0400e0a41e78319b3054c53a5d4e` will be pushed to the `zarf-ref-c74e2e9626da0400e0a41e78319b3054c53a5d4e` branch).  This ensures that this tag will be pushed and received properly by the airgap `git` server.

## Git Reference-Based Git Repository Clone

If you need even more control, Zarf also supports providing full `git` [refspecs](https://git-scm.com/book/en/v2/Git-Internals-The-Refspec), as seen in `https://repo1.dso.mil/big-bang/bigbang.git@refs/heads/release-1.53.x`.  This allows you to pull specific tags or branches by using this standard.  The branch name used by zarf on deploy will depend on the kind of ref specified, branches will use the upstream branch name, whereas other refs (namely tags) will use the `zarf-ref-*` branch name.

## Git Repository Full Clone

Full clones are used in this example with the `stefanprodan/podinfo` repository and follow the `scheme://host/repo` format (`https://github.com/stefanprodan/podinfo.git`). Full clones will contain **all** branches and tags in the mirrored repository rather than any one specific tag.

## Example Usage

This example assumes you have already initialized a Zarf cluster. If that is not the case, refer to the [Initializing the Cluster Walkthrough](../../docs/13-walkthroughs/1-initializing-a-k8s-cluster.md). Be sure when initializing the Zarf cluster to deploy the `git` component, or be ready to specify an external `git` repository.

### Create the Zarf Package

To create this Zarf package run the below command:

``` bash
cd <zarf dir>/examples/git-data    # directory with zarf.yaml
zarf package create                # make the package
```

Successful execution will create a package named `zarf-package-git-data-<arch>-<vx.x.x>.tar.zst`.

### Deploying the Zarf Package

To deploy the Zarf package, copy it to a machine that either has a Zarf cluster deployed with the `git` component or an accessible external repository and the `zarf` executable in your `PATH`.

With the Zarf package in the current working directory, execute the below command to deploy the package, uploading the Git repositories to Gitea and the container images to the Docker registry.

``` bash
zarf package deploy zarf-package-git-data-<arch>-<vx.x.x>.tar.zst
```

:::note

If you are using an external `git` repository you should specify it here with the git url and user flags.

:::

### Applying the Kustomization

Once the package has been deployed, the Kustomization can be applied from the `git` repository using the below command.

:::note

The following assumes you are using the internal Gitea server. If you are using an external server `zarf connect` is not required and you must change the user/url information as needed.*

:::

``` bash
# Run 'zarf connect' and send it to the background
zarf connect git&

# Apply the kustomization
zarf tools kubectl apply -k http://zarf-git-user:$(zarf tools get-admin-password)@localhost:<WhicheverPortGotUsed>/zarf-git-user/mirror__github.com__stefanprodan__podinfo//kustomize

# Inspect
zarf tools k9s

# Bring the connection back to the foreground
fg

# Kill the connection with Ctrl-C
```

## Clean Up

Clean up simply by just deleting the whole cluster

``` bash
kind delete cluster
```
