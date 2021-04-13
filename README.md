# BigBang Utility

This folder contains contents for building BigBang utility clusters. and produces a self-sufficient [`run`](https://makeself.io/) file that when executed, will instantiate a ready-to-use k3s cluster consisting of carefully selected [packages](#packages).

The result is a _extremely_ portable (< 300MB) yet scalable cluster capable of running [almost anywhere](https://k3s.io/) completely airgapped, and can serve as the foundation for further downstream clusters.

## Usage

TODO: Obtain the latest packaged utility cluster from BigBang's releases page, this can either be a `*.run.tgz` or `*.run.zst`.
Or build it using the instructions [below](#build).

Once obtained, transfer it to a host linux machine and run it as root:

```bash
sudo ./bigbang-utility.run.tgz
```

In less than a minute, you'll have a kubernetes cluster running all the pre-requisites needed to host and deploy mutliple other downstream BigBang clusters.

The status of the cluster creation can be monitored in several ways:

```bash
# systemd enabled instances
journalctl -lf -u k3s

# kubectl
watch /usr/local/bin/kubectl get all -A
```

### Override cluster configuration

Sometimes it makes sense to override the default k3s configuration, such as when you know you'll be scaling the cluster out and will need to use the embedded `etcd` instead of `sqlite` backend.

To override the default k3s configuration, place your own config file in `/etc/rancher/k3s/config.yaml` before executing the `run` file.

### Scale cluster

If needed, elastically scale the cluster by adding more servers/agents the same way you would with k3s:

```bash
# on a new node
cat > /etc/rancher/k3s/config.yaml <<EOF
token: "${cluster-token}"
server: "${server-url}"
EOF

sudo ./bigbang-utility.run.tgz
```

## Packages

Only the bare minimum packages are included in the utility cluster in order to stay true to it's minimalist nature.  The following packages are deployed on boot:

* minimal http(s) and/or ssh git server
* OCI compatible container registry
* `metallb` to extend the builtin `k3s` load balancer

The goal of every package _must_ be to provide core capabilities for operators provisioning downstream clusters with BigBang.

## Build

Builds are performed using [earthly](https://earthly.dev/) to ensure an easy to use repeatable build environment is used to produce a single build artifact.

```bash
# assuming earthly and pre-reqs are installed and available on $PATH
earthly +build
```