# Example: Big Bang Umbrella

This example extends the Big Bang Core example package and deploys Big Bang Umbrella with a gitops service. This package deploys both built in addons such as gitlab, mattermost, sonarqube, and nexus, and additional helm charts such as Jira and Confluence. This is not normally the method that will be used in production but for a demo it works great.

Because the same cluster will be running both Traefik and Istio, Istio's VirtualServices will be available on port 9443

<span style="color:red; font-size:2em">NOTE: This package is huge. We recommend not trying to run it on a developer laptop without disabling lots of stuff first.</span>

## Prerequisites

- `make`
- `kustomize`
- `sha256sum`
- TONS of CPU and RAM. Our testing shows the EC2 instance type r6i.4xlarge works pretty well at about $1/hour, which can be reduced further if you do a spot instance.
- [Vagrant](https://www.vagrantup.com/) and [VirtualBox](https://www.virtualbox.org/), only if you are going to use a Vagrant VM, which is incompatible when using an EC2 instance.

Note: Vagrant and VirtualBox aren't required for Zarf to function, but this example's Makefile uses them to create a VM which everything will run in. In production you'll likely just run Zarf on the machine itself.

## Instructions

1. `cd examples/big-bang-umbrella`
1. Run one of these two commands:
   - `make all` - Download the latest version of Zarf, build the deploy package, and start a VM with Vagrant
   - `make all-dev` - Build Zarf locally, build the deploy package, and start a VM with Vagrant. Requires Golang.

     > Note: If you are in an EC2 instance you should skip the `vm-init` make target, so run `make clean fetch-release package-example-big-bang-umbrella && cd ../sync && sudo su` instead, then move on to the next step.
1. Run: `./zarf init --confirm --components management,gitops-service --host 127.0.0.1` - Initialize Zarf, telling it to install the management component and gitops service and skip logging component (since BB has logging already) and tells Zarf to use `127.0.0.1`
1. . If you want to use interactive mode instead just run `./zarf init`.
1. Wait a bit, run `k9s` to see pods come up. Don't move on until everything is running
1. Run: `./zarf package deploy zarf-package-big-bang-umbrella-demo.tar.zst --confirm` - Deploy Big Bang Core. If you want interactive mode instead just run `./zarf package deploy`, it will give you a picker to choose the package.
1. Wait several minutes. Run `k9s` to watch progress
1. :warning: `kubectl delete -n istio-system envoyfilter/misdirected-request` (due to [this bug](https://repo1.dso.mil/platform-one/big-bang/bigbang/-/issues/802))
1. Use a browser to visit the various services, available at https://*.bigbang.dev:9443
1. When you're done, run `exit` to leave the VM then `make vm-destroy` to bring everything down

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
