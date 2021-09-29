# Example: Big Bang Core All-In-One

This example deploys Big Bang Core to a Utility Cluster. This is not normally the method that will be used in production but for a demo it works great.

Because the same cluster will be running both Traefik and Istio, Istio's VirtualServices will be available on port 9443

## Prerequisites

1. Install [Vagrant](https://www.vagrantup.com/)
2. Install `make` and `kustomize`

## Instructions

1. From within the examples directory, Run: `make all`, which will download the latest built binaries, build all of the example packages, and launch a basic VM to run in. Alternatively, run `make all-dev` if you want to build the binaries using the current codebase instead of downloading them.
5. Run: `sudo su` - Change user to root
6. Run: `cd zarf-examples` - Change to the directory where the examples folder is mounted
7. Run: `./zarf init --confirm --features management,utility-cluster --host localhost` - Initialize Zarf, telling it to install the management feature and utility cluster and skip logging feature (since BB has logging already) and tells Zarf to use `localhost` as the domain
8. Wait a bit, run `k9s` to see pods come up. Don't move on until everything is running
9. Run: `./zarf package deploy zarf-package-big-bang-core-demo.tar.zst --confirm` - Deploy Big Bang Core
10. Wait several minutes. Run `k9s` to watch progress
11. :warning: `kubectl delete -n istio-system envoyfilter/misdirected-request` (due to [this bug](https://repo1.dso.mil/platform-one/big-bang/bigbang/-/issues/802))
12. Use a browser to visit the various services, available at https://*.bigbang.dev:9443
13. When you're done, run `make vm-destroy` to bring everything down

## Kubescape scan

This example adds the `kubescape` binary, which can scan clusters for compliance with the NSA/CISA Kubernetes Hardening Guide

```shell
kubescape scan framework nsa --use-from /usr/local/bin/kubescape-framework-nsa.json
```

## Services

| URL                                                   | Username  | Password                                                                                                                                                                                   | Notes                                                               |
| ----------------------------------------------------- | --------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------- |
| [AlertManager](https://alertmanager.bigbang.dev:9443) | n/a       | n/a                                                                                                                                                                                        | Unauthenticated                                                     |
| [Grafana](https://grafana.bigbang.dev:9443)           | `admin`   | `prom-operator`                                                                                                                                                                            |                                                                     |
| [Kiali](https://kiali.bigbang.dev:9443)               | n/a       | `kubectl get secret -n kiali -o=json \| jq -r '.items[] \| select(.metadata.annotations."kubernetes.io/service-account.name"=="kiali-service-account") \| .data.token' \| base64 -d; echo` |                                                                     |
| [Kibana](https://kibana.bigbang.dev:9443)             | `elastic` | `kubectl get secret -n logging logging-ek-es-elastic-user -o=jsonpath='{.data.elastic}' \| base64 -d; echo`                                                                                |                                                                     |
| [Prometheus](https://prometheus.bigbang.dev:9443)     | n/a       | n/a                                                                                                                                                                                        | Unauthenticated                                                     |
| [Jaeger](https://tracing.bigbang.dev:9443)            | n/a       | n/a                                                                                                                                                                                        | Unauthenticated                                                     |
| [Twistlock](https://twistlock.bigbang.dev:9443)       | n/a       | n/a                                                                                                                                                                                        | Twistlock has you create an admin account the first time you log in |
