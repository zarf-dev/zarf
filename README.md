# Shift Pack

This tool creates self-bootstrapping k3s clusters with the requestsed images/manifests embedded to deploy into an airgap Debian or RHEL-based linux environment.  

The result is a _extremely_ portable (< 300MB) yet scalable cluster capable of running [almost anywhere](https://k3s.io/) completely airgapped, and can serve as the foundation for further downstream clusters.


## Prereqs
---
Builds are performed using [earthly](https://earthly.dev/) to ensure an easy to use repeatable build environment is used to produce a single build artifact.  You should also have [docker](https://www.docker.com/products/docker-desktop) installed.  The first thing you will need to do is log into the [Iron Bank](https://registry1.dso.mil/) and you may want to [log into Docker Hub](https://docs.docker.com/engine/reference/commandline/login/) as well if you get throttled.

You'll need your CLI Secret from [User Profile]->[CLI secret] in Harbor to continue.

`docker login registry1.dso.mil`

You will also need to configure the .env file, use the command below to generate a template.  _Note that you don't need to set RHEL creds if you aren't using RHEL_

`earthly +envfile`

## Building
---
To build the packages needed for RHEL-based distros, you will need a Red Hat account (developer accounts are free) to pull the required RPMs for SELINUX-enforcing within the environment.  You must specify the credentials along with the RHEL version flag (7 or 8) in the .env file.  To build the package:

`earthly +build`

---
_note: earthly collects anonymous stats by default, this can be disabled using [these instructions](https://docs.earthly.dev/docs/misc/data-collection#disabling-analytics)_ 

## Deploying
---
You can try out the deployment using vagrant:

```bash
vagrant destroy -f && vagrant up --provision --no-color && vagrant ssh [RHEL7|Ubuntu]
```

In less than a minute, you'll have a kubernetes cluster running all the pre-requisites needed to host and deploy mutliple other downstream clusters.

The status of the cluster creation can be monitored in several ways:

```bash
# systemd enabled instances
journalctl -lf -u k3s

# kubectl
watch kubectl get no,all -A
```

## Scaling
---
If needed, elastically scale the cluster by adding more servers/agents the same way you would with k3s:

```bash
# on a new node
cat > /etc/rancher/k3s/config.yaml <<EOF
token: "${cluster-token}"
server: "${server-url}"
EOF

sudo ./shift-pack initialize
```
