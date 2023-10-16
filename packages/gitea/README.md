## Zarf Git Server

This package contains the Zarf Git Server to enable more advanced gitops-based deployments. See the [git-data](../../examples/git-data/) example for more on how Zarf handles `git` repositories.

### Image Values

The default setup for this package is to use a `rootless` image, specified in the [gitea helm values](./gitea-values.yaml#L49). Because the gitea helm chart does its own appending of `-rootless` to the image tag, based on the `rootless` helm value, users don't need to supply the full image tag when overriding the default gitea image. Instead you need to use the `GITEA_SERVER_VERSION`, either in the zarf-config.toml or with `--set`.

_Make sure, though, that the `x.x.x-rootless` tag does exist for Zarf to find._

```
$ zarf package create . --set GITEA_IMAGE="registry1.dso.mil/ironbank/opensource/go-gitea/gitea" \
  --set GITEA_SERVER_VERSION="v1.19.3"
```
