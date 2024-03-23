import ExampleYAML from "@components/ExampleYAML.astro";

# Git Repositories

This example shows how to package `git` repositories within a Zarf package.  This package does not deploy anything itself but pushes assets to the specified `git` service to be consumed as desired.  Within Zarf, there are a few ways to include `git` repositories (as described below).

:::tip

Git repositories included in a package can be deployed with `zarf package deploy` if an existing Kubernetes cluster has been initialized with `zarf init`.  If you do not have an initialized cluster but want to push resources to a remote registry anyway, you can use [`zarf package mirror-resources`](./../../docs/2-the-zarf-cli/100-cli-commands/zarf_package_mirror-resources.md).

:::

## Tag-Based Git Repository Clone

Tag-based `git` repository cloning is the **recommended** way of cloning a `git` repository for air-gapped deployments because it wraps meaning around a specific point in git history that can easily be traced back to the online world. Tag-based clones are defined using the `scheme://host/repo@tag` format as seen in the example of the `defenseunicorns/zarf` repository (`https://github.com/defenseunicorns/zarf.git@v0.15.0`).

A tag-based clone only mirrors the tag defined in the Zarf definition. The tag will be applied on the `git` mirror to a zarf-specific branch name based on the tag name (e.g. the tag `v0.1.0` will be pushed to the `zarf-ref-v0.1.0` branch).  This ensures that this tag will be pushed and received properly by the airgap `git` server.

:::note

If you would like to use a protocol scheme other than http/https, you can do so with something like the following: `ssh://git@github.com/defenseunicorns/zarf.git@v0.15.0`.  Using this you can also clone from a local repo to help you manage larger git repositories: `file:///home/zarf/workspace/zarf@v0.15.0`.

:::

:::caution

Because Zarf creates long-lived mirrors of repositories in the air gap, it does not support shallow clones (i.e. `git clone --depth x`).  These may be present in build environments (i.e. [GitLab runners](https://github.com/defenseunicorns/zarf/issues/1698)) and should be avoided.  To learn more about shallow and partial clones see the [GitHub blog on the topic](https://github.blog/2020-12-21-get-up-to-speed-with-partial-clone-and-shallow-clone).

:::

## SHA-Based Git Repository Clone

In addition to tags, Zarf also supports cloning and pushing a specific SHA hash from a `git` repository, but this is **not recommended** as it is less readable/understandable than tag cloning.  Commit SHAs are defined using the same `scheme://host/repo@shasum` format as seen in the example of the `defenseunicorns/zarf` repository (`https://github.com/defenseunicorns/zarf.git@c74e2e9626da0400e0a41e78319b3054c53a5d4e`).

A SHA-based clone only mirrors the SHA hash defined in the Zarf definition. The SHA will be applied on the `git` mirror to a zarf-specific branch name based on the SHA hash (e.g. the SHA `c74e2e9626da0400e0a41e78319b3054c53a5d4e` will be pushed to the `zarf-ref-c74e2e9626da0400e0a41e78319b3054c53a5d4e` branch).  This ensures that this tag will be pushed and received properly by the airgap `git` server.

## Git Reference-Based Git Repository Clone

If you need even more control, Zarf also supports providing full `git` [refspecs](https://git-scm.com/book/en/v2/Git-Internals-The-Refspec), as seen in `https://repo1.dso.mil/big-bang/bigbang.git@refs/heads/release-1.54.x`.  This allows you to pull specific tags or branches by using this standard.  The branch name used by zarf on deploy will depend on the kind of ref specified, branches will use the upstream branch name, whereas other refs (namely tags) will use the `zarf-ref-*` branch name.

## Git Repository Full Clone

Full clones are used in this example with the `stefanprodan/podinfo` repository and follow the `scheme://host/repo` format (`https://github.com/stefanprodan/podinfo.git`). Full clones will contain **all** branches and tags in the mirrored repository rather than any one specific tag.

:::note

If you want to learn more about how Zarf works with GitOps, see the [podinfo-flux](../podinfo-flux/) example.

:::

## `zarf.yaml` {#zarf.yaml}

:::note

To view the example in its entirety, select the `Edit this page` link below the article and select the parent folder.

:::

<ExampleYAML src={require('./zarf.yaml')} showLink={false} />
