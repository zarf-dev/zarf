The default setup for this package is to use a `rootless` image, specified in `gitea-values.yaml`. Because the gitea helm chart does its own appending of `-rootless` to the image tag, based on the `rootless` helm value, users don't need to supply the full image tag when overriding the default gitea image. Instead you need to use the `GITEA_SERVER_VERSION`, either in the zarf-config.toml or with `--set`.

The `zarf-git-server-tls` chart in this package creates the Secret mounted by Gitea. By default it generates a CA and server certificate, and preserves them across package upgrades. To supply your own certificate during init, set all three file variables `GIT_SERVER_TLS_CA`, `GIT_SERVER_TLS_CERT`, and `GIT_SERVER_TLS_KEY` to PEM file paths. Supplying certificate files enables TLS. The chart stores the resulting certificate and key in `zarf-git-server-tls`; Zarf reads the Secret when it distributes the CA to Git clients or detects certificate changes. Helm release metadata also contains the Secret manifest and supplied values, so access to Helm release Secrets must be restricted. Use `zarf tools update-creds git` for later rotations.

_Make sure, though, that the `x.x.x-rootless` tag does exist for Zarf to find._

```bash
$ zarf package create . --set GITEA_IMAGE="custom.enterprise.corp/ironbank/opensource/gitea" \
--set GITEA_SERVER_VERSION="v1.19.3"
```
