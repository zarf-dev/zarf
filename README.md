# Zarf - DevSecOps for Air Gap

[![Latest Release](https://img.shields.io/github/v/release/defenseunicorns/zarf)](https://github.com/defenseunicorns/zarf/releases)
[![Zarf Slack Channel](https://img.shields.io/badge/k8s%20slack-zarf-40a3dd)](https://kubernetes.slack.com/archives/C03B6BJAUJ3)
[![Zarf Website](https://img.shields.io/badge/web-zarf.dev-6d87c3)](https://zarf.dev/)
[![Zarf Documentation](https://img.shields.io/badge/docs-docs.zarf.dev-775ba1)](https://docs.zarf.dev/)
[![Go version](https://img.shields.io/github/go-mod/go-version/defenseunicorns/zarf?filename=go.mod)](https://go.dev/)
[![Build Status](https://img.shields.io/github/workflow/status/defenseunicorns/zarf/Publish%20Zarf%20Packages%20on%20Tag)](https://github.com/defenseunicorns/zarf/actions/workflows/release.yml)

<img align="right" alt="zarf logo" src=".images/zarf-logo.png"  height="256" />

Zarf eliminates the [complexity of air gap software delivery](https://www.itopstimes.com/contain/air-gap-kubernetes-considerations-for-running-cloud-native-applications-without-the-cloud/) for Kubernetes clusters and cloud native workloads using a declarative packaging strategy to support DevSecOps in offline and semi-connected environments.

📦 Out of the Box Features

- Automate Kubernetes deployments in disconnected environments
- Automate [Software Bill of Materials (SBOM)](https://www.linuxfoundation.org/tools/the-state-of-software-bill-of-materials-sbom-and-cybersecurity-readiness/) generation
- Provide a [web dashboard](https://docs.zarf.dev/docs/dashboard-ui/sbom-dashboard) for viewing SBOM output
- Deploy a new cluster while fully disconnected with [K3s](https://k3s.io/) or into any existing cluster using a [kube config](https://kubernetes.io/docs/concepts/configuration/organize-cluster-access-kubeconfig/)
- Builtin logging stack with [Loki](https://grafana.com/oss/loki/)
- Builtin git server with [Gitea](https://gitea.com/)
- Builtin docker registry
- Builtin [K9s Dashboard](https://k9scli.io/) for managing a cluster from the terminal
- [Mutating Webhook](adr/0005-mutating-webhook.md) to automatically update Kubernetes pods image path and pull secrets as well as [Flux Git Repository](https://fluxcd.io/docs/components/source/gitrepositories/) URLs and secret references
- Builtin [command to find images](https://docs.zarf.dev/docs/user-guide/the-zarf-cli/cli-commands/zarf_prepare_find-images) and resources from a helm chart
- Tunneling capability to [connect to Kuberenetes resources](https://docs.zarf.dev/docs/user-guide/the-zarf-cli/cli-commands/zarf_connect) without network routing, DNS, TLS or Ingress configuration required

🛠️ Configurable Features

- Customizable [packages variables](examples/package-variables/README.md) with defaults and user prompting
- [Composable packages](https://docs.zarf.dev/docs/user-guide/zarf-packages/zarf-components#composing-package-components) to include multiple sub-packages/components
- Filters to select the correct architectures/operating systems for packages

> Early Zarf research and prototypes were developed jointly with [United States Naval Postgraduate School](https://nps.edu/) research you can read [here](https://calhoun.nps.edu/handle/10945/68688).

## Demo

[![preview](.images/zarf-v0.21-preview.gif)](https://www.youtube.com/watch?v=WnOYlFVVKDE)

_https://www.youtube.com/watch?v=WnOYlFVVKDE_

## Getting Started

To try Zarf out for yourself, visit the ["Try It Now"](https://zarf.dev/install) section on our website, and if you want to learn more about Zarf and its use cases visit [docs.zarf.dev](https://docs.zarf.dev/docs/zarf-overview).

From the docs you can learn more about [installation](https://docs.zarf.dev/docs/operator-manual/set-up-and-install), [using the CLI](https://docs.zarf.dev/docs/user-guide/the-zarf-cli/), [making packages](https://docs.zarf.dev/docs/user-guide/zarf-packages/), and the [Zarf package schema](https://docs.zarf.dev/docs/user-guide/zarf-schema).

## Developing

To contribute, please see our [Contributor Guide](https://docs.zarf.dev/docs/developer-guide/contributor-guide).  Below is an architectural diagram showing the basics of how Zarf functions which you can read more about [here](https://docs.zarf.dev/docs/developer-guide/nerd-notes).

![Architecture Diagram](./docs/architecture.drawio.svg)

[Source DrawIO](docs/architecture.drawio.svg)

## Special Thanks

We would also like to thank the following awesome libraries and projects without which Zarf would not be possible!

[![pterm/pterm](https://img.shields.io/badge/pterm%2Fpterm-007d9c?logo=go&logoColor=white)](https://github.com/pterm/pterm)
[![mholt/archiver](https://img.shields.io/badge/mholt%2Farchiver-007d9c?logo=go&logoColor=white)](https://github.com/mholt/archiver)
[![spf13/cobra](https://img.shields.io/badge/spf13%2Fcobra-007d9c?logo=go&logoColor=white)](https://github.com/spf13/cobra)
[![go-git/go-git](https://img.shields.io/badge/go--git%2Fgo--git-007d9c?logo=go&logoColor=white)](https://github.com/go-git/go-git)
[![sigstore/cosign](https://img.shields.io/badge/sigstore%2Fcosign-2a1e71?logo=linuxfoundation&logoColor=white)](https://github.com/sigstore/cosign)
[![helm.sh/helm](https://img.shields.io/badge/helm.sh%2Fhelm-0f1689?logo=helm&logoColor=white)](https://github.com/helm/helm)
[![kubernetes](https://img.shields.io/badge/kubernetes-316ce6?logo=kubernetes&logoColor=white)](https://github.com/kubernetes)
