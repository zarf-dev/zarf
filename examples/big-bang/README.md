# Example: Big Bang Core All-In-One

This example deploys Big Bang Core with a gitops service. This is not normally the method that will be used in production but for a demo it works great.

Because the same cluster will be running both Traefik and Istio, Istio's VirtualServices will be available on port 9443

## Prerequisites

1. Install [Vagrant](https://www.vagrantup.com/)
2. Install `make` and `kustomize`
1. Install `sha256sum` (on Mac it's `brew install coreutils`)

## Instructions

1. `cd examples/big-bang`
1. Run one of these two commands:
   - `make all` - Download the latest version of Zarf, build the deploy package, and start a VM with Vagrant
   - `make all-dev` - Build Zarf locally, build the deploy package, and start a VM with Vagrant
1. Run: `./zarf init --confirm --components management,gitops-service --host localhost` - Initialize Zarf, telling it to install the management component and gitops service and skip logging component (since BB has logging already) and tells Zarf to use `localhost` as the domain. If you want to use interactive mode instead just run `./zarf init`.
1. Wait a bit, run `k9s` to see pods come up. Don't move on until everything is running
1. Run: `./zarf package deploy zarf-package-big-bang-core-demo.tar.zst --confirm` - Deploy Big Bang Core. If you want interactive mode instead just run `./zarf package deploy`, it will give you a picker to choose the package.
1. Wait several minutes. Run `k9s` to watch progress
1. :warning: `kubectl delete -n istio-system envoyfilter/misdirected-request` (due to [this bug](https://repo1.dso.mil/platform-one/big-bang/bigbang/-/issues/802))
1. Use a browser to visit the various services, available at https://*.bigbang.dev:9443
1. When you're done, run `make vm-destroy` to bring everything down

## Kubescape scan

This example adds the `kubescape` binary, which can scan clusters for compliance with the NSA/CISA Kubernetes Hardening Guide

```shell
kubescape scan framework nsa --use-from=/usr/local/bin/kubescape-framework-nsa.json --exceptions=/usr/local/bin/kubescape-exceptions.json
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
