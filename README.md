# Zarf - DevSecOps for Air Gap

[![Latest Release](https://img.shields.io/github/v/release/defenseunicorns/zarf)](https://github.com/defenseunicorns/zarf/releases)
[![Zarf Slack Channel](https://img.shields.io/badge/k8s%20slack-zarf-6d87c3)](https://kubernetes.slack.com/archives/C03B6BJAUJ3)
[![Zarf Documentation](https://img.shields.io/badge/web-zarf.dev-775ba1)](https://zarf.dev/)
[![Go version](https://img.shields.io/github/go-mod/go-version/:user/:repo?filename=go.mod)](https://go.dev/)

<img align="right" alt="zarf logo" src=".images/zarf-logo.png"  height="256" />

Zarf simplifies the setup & administration of Kubernetes clusters, cyber systems & workloads that support DevSecOps "across the [air gap](https://en.wikipedia.org/wiki/Air_gap_(networking))."

It provides a static go binary (can run anywhere) CLI that can pull, package, and install everything a cluster needs to run alongside any necessary infrastructure resources like Terraform. Zarf also caches downloads (for speed), hashes packages (for security), and can _even install the Kubernetes cluster itself_ if you needed.

Zarf runs on [a bunch of operating systems](./docs/supported-oses.md), and aims to support configurations ranging from "I want to run one, simple app" to "I need to support & dependency control a _bunch_ of internet-disconnected clusters."

Zarf was initially theorized and demonstrated in research from Naval Postgraduate School which you can read [here](https://calhoun.nps.edu/handle/10945/68688).

## Demo

[![asciicast](https://asciinema.org/a/475530.svg)](https://asciinema.org/a/475530)

## Docs

To learn more about Zarf and its use cases visit [docs.zarf.dev](https://docs.zarf.dev/docs/zarf-overview).

From there you can learn about [installation](https://docs.zarf.dev/docs/operator-manual/set-up-and-install), [using the CLI](https://docs.zarf.dev/docs/user-guide/the-zarf-cli/), [making packages](https://docs.zarf.dev/docs/user-guide/zarf-packages/), and the [Zarf package schema](https://docs.zarf.dev/docs/user-guide/zarf-schema).

<!-- TODO Copy editing -->

## Developing

To contribute, please see our [Contributor Guide](https://docs.zarf.dev/docs/developer-guide/contributor-guide).  Below is an architectural diagram showing the basics of how Zarf functions which you can read more about [here](https://docs.zarf.dev/docs/developer-guide/nerd-notes).

![Architecture Diagram](./docs/architecture.drawio.svg)

[Source DrawIO](docs/architecture.drawio.svg)

<!-- TODO Copy editing -->

## Special Thanks

Zarf would not be possible without the people behind these awesome libraries and projects!

[![pterm/pterm](https://img.shields.io/badge/pterm%2Fpterm-007d9c?logo=go&logoColor=white)](https://github.com/pterm/pterm)
[![mholt/archiver](https://img.shields.io/badge/mholt%2Farchiver-007d9c?logo=go&logoColor=white)](https://github.com/mholt/archiver)
[![spf13/cobra](https://img.shields.io/badge/spf13%2Fcobra-007d9c?logo=go&logoColor=white)](https://github.com/spf13/cobra)
[![go-git/go-git](https://img.shields.io/badge/go--git%2Fgo--git-007d9c?logo=go&logoColor=white)](https://github.com/go-git/go-git)
[![sigstore/cosign](https://img.shields.io/badge/sigstore%2Fcosign-2a1e71?logo=linuxfoundation&logoColor=white)](https://github.com/sigstore/cosign)
[![helm.sh/helm](https://img.shields.io/badge/helm.sh%2Fhelm-0f1689?logo=helm&logoColor=white)](https://github.com/helm/helm)
[![kubernetes](https://img.shields.io/badge/kubernetes-316ce6?logo=kubernetes&logoColor=white)](https://github.com/kubernetes)

<!-- TODO Formatting/Finalizing -->
