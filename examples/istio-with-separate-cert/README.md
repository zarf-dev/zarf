# Example: Big Bang's Istio with a separately loaded cert

This example deploys Big Bang's Istio, but without an ingress cert. It is applicable in use cases where you want to have a freely distributable zarf package, but your ingress cert is private and can't be distributed in the same way that you want the Zarf package to be.

Because the same cluster will be running both Traefik and Istio, Istio's VirtualServices will be available on port 9443

## Prerequisites

1. Install [Vagrant](https://www.vagrantup.com/)
2. Install `make` and `kustomize`
1. Install `sha256sum` (on Mac it's `brew install coreutils`)
1. Be logged into Registry1. If you can run `docker pull registry1.dso.mil/ironbank/big-bang/base:8.4` successfully you're good to go.

## Instructions

1. `cd examples/istio-with-separate-cert`
1. Run one of these two commands:
   - `make all` - Download the latest version of Zarf, build the deploy package, and start a VM with Vagrant
   - `make all-dev` - Build Zarf locally, build the deploy package, and start a VM with Vagrant
1. Run: `./zarf init --confirm --components management,gitops-service --host 127.0.0.1` - Initialize Zarf, telling it to install the management component and gitops service and skip logging component (since BB has logging already) and tells Zarf to use `127.0.0.1` (which is the same as localhost) as the hostname. If you want to use interactive mode instead just run `./zarf init`.
1. Wait a bit, run `k9s` to see pods come up. Don't move on until everything is running
1. Run: `./zarf package deploy zarf-package-istio-with-separate-cert.tar.zst --confirm` - Deploy the package. If you want interactive mode instead just run `./zarf package deploy`, it will give you a picker to choose the package.
1. Wait for a couple of minutes for everything to come up. Run `k9s` to watch progress
1. :warning: Once Istio is running, run `kubectl delete -n istio-system envoyfilter/misdirected-request` (due to [this bug](https://repo1.dso.mil/platform-one/big-bang/bigbang/-/issues/802))
1. Try to hit Kiali at https://kiali.bigbang.dev:9443. It won't work, because you haven't applied a valid cert yet
1. The cert was copied into the `sync` folder that is mounted in the VM when you ran `make all` or `make all-dev`. Now it's time to apply it. Run `kubectl apply -f secret-tls.yaml`
1. Use a browser to visit the various services, available at https://*.bigbang.dev:9443
1. When you're done, run `exit` to leave the VM then `make vm-destroy` to bring everything down

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
