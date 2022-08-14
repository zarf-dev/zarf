# Zarf - DevSecOps for Air Gap Systems

[![Latest Release](https://img.shields.io/github/v/release/defenseunicorns/zarf)](https://github.com/defenseunicorns/zarf/releases)
[![Zarf Slack Channel](https://img.shields.io/badge/k8s%20slack-zarf-6d87c3)](https://kubernetes.slack.com/archives/C03B6BJAUJ3)
[![Zarf Documentation](https://img.shields.io/badge/web-zarf.dev-b99cca)](https://zarf.dev/)

<img align="right" alt="zarf logo" src=".images/zarf-logo.png"  height="256" />

Zarf simplifies the setup & administration of Kubernetes clusters, cyber systems & workloads that support DevSecOps "across the [air gap](https://en.wikipedia.org/wiki/Air_gap_(networking))."

It provides a static go binary (can run anywhere) CLI that can pull, package, and install everything a cluster needs to run alongside any necessary infrastructure resources like Terraform. Zarf also caches downloads (for speed), hashes packages (for security), and can _even install the Kubernetes cluster itself_ if you needed.

Zarf runs on [a bunch of operating systems](./docs/supported-oses.md), and aims to support configurations ranging from "I want to run one, simple app" to "I need to support & dependency control a _bunch_ of internet-disconnected clusters."

Zarf was theorized and initially demonstrated in Naval Postgraduate School research you can read about [here](https://calhoun.nps.edu/handle/10945/68688).

## Demo

[![asciicast](https://asciinema.org/a/475530.svg)](https://asciinema.org/a/475530)

## Docs

To learn more about Zarf... <!-- TODO -->

## Contributing

To contribute to Zarf... <!-- TODO -->

### Architecture
![Architecture Diagram](./docs/architecture.drawio.svg)

[Source DrawIO](docs/architecture.drawio.svg)

## Special Thanks

Zarf would not be possible without the people behind these awesome libraries.

<!-- TODO -->
