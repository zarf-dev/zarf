# Zarf - DevSecOps for Air Gap

[![Latest Release](https://img.shields.io/github/v/release/defenseunicorns/zarf)](https://github.com/defenseunicorns/zarf/releases)
[![Zarf Slack Channel](https://img.shields.io/badge/k8s%20slack-zarf-40a3dd)](https://kubernetes.slack.com/archives/C03B6BJAUJ3)
[![Zarf Website](https://img.shields.io/badge/web-zarf.dev-6d87c3)](https://zarf.dev/)
[![Zarf Documentation](https://img.shields.io/badge/docs-docs.zarf.dev-775ba1)](https://docs.zarf.dev/)
[![Go version](https://img.shields.io/github/go-mod/go-version/defenseunicorns/zarf?filename=go.mod)](https://go.dev/)
[![Build Status](https://img.shields.io/github/workflow/status/defenseunicorns/zarf/Publish%20Zarf%20Packages%20on%20Tag)](https://github.com/defenseunicorns/zarf/actions/workflows/release.yml)

<img align="right" alt="zarf logo" src=".images/zarf-logo.png"  height="256" />

Zarf simplifies the setup & administration of Kubernetes clusters, cyber systems & workloads that support DevSecOps "across the [air gap](https://en.wikipedia.org/wiki/Air_gap_(networking))."

📦 Out of the Box Features

- Automate Kubernetes deployments in disconnected environments
- Automate Software Bill of Materials (SBOM) generation
- Provide HTML Dashboards for viewing SBOM output
- Deploy a new cluster while fully disconnected (using K3s)
- Deploy pre-built tar.zst package into any existing cluster (using the kubernetes context)
- Builtin logging (PLG) and seedable git repository (gitea) and docker registry
- Automatically update pod's ImagePullSecrets so resources use the NodePort (See the Zarf Agent)
- Builtin K9s Dashboard for visualizing containers and clusters
- Builtin command to find images and resources from a helm chart
- Create secure tunnel ports for deployments

🛠️ Configurable Features

- Customizable packages variables with defaults and user prompting
- Composable packages to include multiple sub-packages/components
- Filters to select the correct architectures/operating systems for packages

Zarf was initially theorized and demonstrated in research from Naval Postgraduate School which you can read [here](https://calhoun.nps.edu/handle/10945/68688).

## Demo

[![asciicast](https://asciinema.org/a/475530.svg)](https://asciinema.org/a/475530)

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
