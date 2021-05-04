# Shift Pack

This tool creates self-bootstrapping k3s clusters with the requestsed images/manifests embedded to deploy into an airgap Debian or RHEL-based linux environment.  

The result is a _extremely_ portable (< 300MB) yet scalable cluster capable of running [almost anywhere](https://k3s.io/) completely airgapped, and can serve as the foundation for further downstream clusters.

&nbsp;

## Prereqs

### Software
To run this tool, you'll need some software pre-installed:

- [Earthly](https://earthly.dev/) : Ensures a repeatable, easy-to-use build environment to produce your build artifacts.

- [Docker](https://www.docker.com/products/docker-desktop) : Provides access to secure build images and assists earthly in keeping builds self-contained, isolated, and repeatable.

### User Accounts
This tool utilizes software pulled from multiple sources and _some_ of them require authenticated access.  You will need to make an account at the following sites if you don't already have access:

- [Iron Bank](https://registry1.dso.mil/) : Platform One's authorized, hardened, and approved container repository. ([product](https://p1.dso.mil/#/products/iron-bank/) [pages](https://ironbank.dso.mil/))

- [RedHat Developer](https://developers.redhat.com/) : RedHat's developer access program which allows access to their (normally) for-pay software & services.
  - Access account creation via: Menu > Login > Create one now.
  - This project runs perfectly well using a "_Developer Subscription for Individuals_" (which is free!).

  ---

  **NOTE**

  You only need a RedHat Dev account if you plan on building on a RHEL distro!

  ---

&nbsp;

You will also need to configure the .env file, use the command below to generate a template.  _Note that you don't need to set RHEL creds if you aren't using RHEL_

`earthly +envfile`

## Building
---
To build the packages needed for RHEL-based distros, you will need a Red Hat account (developer accounts are free) to pull the required RPMs for SELINUX-enforcing within the environment.  You must specify the credentials along with the RHEL version flag (7 or 8) in the .env file.  To build the package:

### Step 1b - Configure the `.env` file

Some secrets also have to be passed to Earthly for your build, these are stored in the `.env` file.  YOu can generate a template to complete with the command below. 

`earthly +envfile`

_To build the packages needed for RHEL-based distros, you will need to use your RedHat Developer account to pull the required RPMs for SELINUX-enforcing within the environment.  You must specify your credentials along with a RHEL version flag (7 or 8) in the `.env` file_

&nbsp;

### Step 2 - Run a Build

Building the package is one command:

```sh
earthly +build
```

---

***NOTE***

Earthly collects anonymous stats by default but that [can be disabled ](https://docs.earthly.dev/docs/misc/data-collection#disabling-analytics) if you don't like it.

---

&nbsp;

### Step 3 - Test Drive

You can try out your new build with a local [Vagrant](https://www.vagrantup.com/) deployment, like so:

```bash
# bring up VM on host & connect into it
vagrant destroy -f && vagrant up --provision --no-color && vagrant ssh [RHEL7|Ubuntu]

# escalate user once inside VM: vagrant --> root
sudo su
```

In less than a minute, you'll have a kubernetes cluster running all the pre-requisites needed to host and deploy mutliple other downstream clusters.

The status of the cluster creation can be monitored in several ways:

```bash
# systemd enabled instances
journalctl -lf -u k3s

# kubectl
watch /usr/local/bin/kubectl get no,all -A
```
If needed, elastically scale the cluster by adding more servers/agents the same way you would with k3s:

```bash
# on a new node
cat > /etc/rancher/k3s/config.yaml <<EOF
token: "${cluster-token}"
server: "${server-url}"
EOF

sudo ./shift-pack initialize
```

&nbsp;

### Step 4 - Cleanup

You can tear down the local [Vagrant](https://www.vagrantup.com/) deployment, like so:

```bash
# to deescalate user: root --> vagrant
exit

# to exit VM shell
exit

# tear down the VM
vagrant destroy -f
```
