# Example: Big Bang Umbrella

This example extends the Big Bang Core example package and deploys Big Bang Umbrella with a gitops service. This package deploys both built in addons such as gitlab, mattermost, sonarqube, and nexus, and additional helm charts such as Jira and Confluence. This is not normally the method that will be used in production but for a demo it works great.

Because the same cluster will be running both Traefik and Istio, Istio's VirtualServices will be available on port 9443

<span style="color:red; font-size:2em">NOTE: This package is huge. We recommend not trying to run it on a developer laptop without disabling lots of stuff first.</span>

## Prerequisites

- [Vagrant](https://www.vagrantup.com/)
- [VirtualBox](https://www.virtualbox.org/)
- `make`
- `kustomize`
- `sha256sum`
- TONS of CPU and RAM. Our testing shows the EC2 instance type r6i.4xlarge works pretty well at about $1/hour, which can be reduced further if you do a spot instance.

Note: Vagrant and VirtualBox aren't required for Zarf to function, but this example's Makefile uses them to create a VM which everything will run in. In production you'll likely just run Zarf on the machine itself.

## Instructions

1. From within the examples directory, Run: `make all`, which will download the latest built binaries, build all of the example packages, and launch a basic VM to run in. Alternatively, run `make all-dev` if you want to build the binaries using the current codebase instead of downloading them.
5. Run: `sudo su` - Change user to root
6. Run: `cd zarf-examples` - Change to the directory where the examples folder is mounted
7. Run: `./zarf init --confirm --components management,gitops-service --host 127.0.0.1` - Initialize Zarf, telling it to install the management component and gitops service and skip logging component (since BB has logging already) and tells Zarf to use `127.0.0.1` as the domain
8. Wait a bit, run `k9s` to see pods come up. Don't move on until everything is running
10. Run: `./zarf package deploy zarf-package-big-bang-umbrella-demo.tar.zst --confirm` - Deploy Big Bang Umbrella
11. Wait several minutes. Run `k9s` to watch progress
12. :warning: `kubectl delete -n istio-system envoyfilter/misdirected-request` (due to [this bug](https://repo1.dso.mil/platform-one/big-bang/bigbang/-/issues/802))
13. Use a browser to visit the various services, available at https://*.bigbang.dev:9443
14. When you're done, run `make vm-destroy` to bring everything down

NOTE: If you are not running in a Vagrant box created with the Vagrantfile in ./examples you will have to run `sysctl -w vm.max_map_count=262144` to get ElasticSearch to start correctly.

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
| [Twistlock](https://twistlock.bigbang.dev:9443)       | n/a       | n/a                                                                                                                                                                                        | |
| [Jira](https://jira.bigbang.dev:9443)       | n/a       | n/a                                                                                                                                                                                        | |
| [Confluence](https://confluence.bigbang.dev:9443)       | n/a       | n/a                                                                                                                                                                                        | |
| [GitLab](https://gitlab.bigbang.dev:9443)       | n/a       | n/a                                                                                                                                                                                        |  |
| [Nexus](https://nexus.bigbang.dev:9443)       | n/a       | n/a                                                                                                                                                                                        | |
| [Mattermost](https://chat.bigbang.dev:9443)       | n/a       | n/a                                                                                                                                                                                        | |
| [Sonarqube](https://sonarqube.bigbang.dev:9443)       | n/a       | n/a                                                                                                                                                                                        | |
