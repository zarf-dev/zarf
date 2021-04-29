# Shift Package

This tool creates self-bootstrapping k3s clusters with the requestsed images/manifests embedded to deploy into an airgap Debian or RHEL-based linux environment.  

The result is a _extremely_ portable (< 300MB) yet scalable cluster capable of running [almost anywhere](https://k3s.io/) completely airgapped, and can serve as the foundation for further downstream clusters.

## Usage

Builds are performed using [earthly](https://earthly.dev/) to ensure an easy to use repeatable build environment is used to produce a single build artifact.

To build the packages needed for RHEL-based distros, you will need a Red Hat account (developer accounts are free) to pull the required RPMs for SELINUX-enforcing within the environment.  You must specify the credentials along with the RHEL version flag (7 or 8).

`earthly -s RHEL_USER=*** -s RHEL_PASS=*** --build-arg RHEL="7" +build`

If you don't need to support RHEL-based distros, a simpler command will work.

`earthly +build`

You can try out the deployment using vagrant:

```bash
vagrant destroy -f && vagrant up --provision && vagrant ssh
cd /opt/shift
sudo ./shift-package initialize
```

In less than a minute, you'll have a kubernetes cluster running all the pre-requisites needed to host and deploy mutliple other downstream clusters.

The status of the cluster creation can be monitored in several ways:

```bash
# systemd enabled instances
journalctl -lf -u k3s

# kubectl
watch kubectl get no,all -A
```

### Scale cluster

If needed, elastically scale the cluster by adding more servers/agents the same way you would with k3s:

```bash
# on a new node
cat > /etc/rancher/k3s/config.yaml <<EOF
token: "${cluster-token}"
server: "${server-url}"
EOF

sudo ./shift-package initialize
```
