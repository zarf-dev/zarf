# 22. Multi-Node Kubernetes with `zarf init`

Date: 2024-01-23

## Status

Pending

## Context

Currently today, when allowing `zarf init` to handle cluster creation, Zarf doesn't have ability to automatically or semi-automatically provision itself across multiple nodes.  The idea here would be to allow for horizontal scalability across multiple virtual or physical nodes for site reliability and automatic failover.

The first pass would consider scaling horizontally with the Embedded DB model.  There would be minimal changes on the current k3s.service config.  The change required here would be to include a shared token.  By default, if K3S doesn't receive a token it will auto generate one.  You can reset this on an existing cluster by running `k3s token rotate --new-token=foo`.

If one wanted to specify a token in advance they could simply modify their existing `zarf deploy/init`, `--set K3S_ARGS` command to include `--token=foo`

For example:
```shell
zarf init --components=k3s,git-server --confirm --set K3S_ARGS=\"--disable=traefik --disable=metrics-server --disable=servicelb --tls-san=1.2.3.4 --token=foo\""
```

This results in a line as such for example:

```ini
ExecStart=/usr/sbin/k3s server --write-kubeconfig-mode=700 --write-kubeconfig /root/.kube/config --disable=traefik --disable=metrics-server --disable=servicelb --tls-san=1.2.3.4 --token=foo
```

The difference on the agent side requires a few changes.  We must specify three pieces of information:

* That we want to spin up a K3S agent only, not any other Zarf components.
* The IP of the `server`.
* The shared token specified when creating the `server`.

This would need to be the results k3s.service file.

```ini
ExecStart=/usr/sbin/k3s agent --server=https://1.2.3.4:6443 --token=foo
```

One approach could be to introduce constants into [k3s.service](packages/distros/k3s/common/k3s.service), that would allow us to reuse it.  A new component would essentially set some of those variables.

For example:

| Variable                        | Server                                            | Agent   |
|---------------------------------|---------------------------------------------------|---------|
| `###ZARF_CONST_K3S_MODE###`     | `server`                                          | `agent` |
| `###ZARF_CONST_K3S_INTERNAL###` | ` --write-kubeconfig-mode=700 --write-kubeconfig` | empty   |
 | `###ZARF_VAR_K3S_ARGS###`       | `--token=foo`                                     | `--server https://1.2.3.4:6443 --token=foo` |

The new k3s.service file would look like:

```init
ExecStart=/usr/sbin/k3s ###ZARF_CONST_K3S_MODE######ZARF_CONST_K3S_INTERNAL### ###ZARF_VAR_K3S_ARGS###
```

References:

* https://github.com/defenseunicorns/zarf-package-bare-metal
* https://github.com/defenseunicorns/zarf/issues/1002
* https://docs.k3s.io/datastore/ha-embedded
* https://docs.k3s.io/cli/agent

## Decision

TBD

## Consequences

...
