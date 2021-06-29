# Zarf K8s Airgap Thingy

This tool creates self-bootstrapping k3s clusters with the requestsed images/manifests embedded to deploy into an airgap Debian or RHEL-based linux environment.  

The result is a portable cluster capable of running [almost anywhere](https://k3s.io/) completely airgapped, and can serve as the foundation for further downstream clusters.

## What's with the name?
### Basically this...
![zarf definition](.images/zarf-dod.jpg)


## Quick Demo

[![asciicast](https://asciinema.org/a/ua6O4JHCy6LT2eXEy78QvbbfC.svg)](https://asciinema.org/a/ua6O4JHCy6LT2eXEy78QvbbfC)

## Prereqs

### Software
To run this tool, you'll need some software pre-installed:

- [Docker](https://www.docker.com/products/docker-desktop) : Provides access to secure build images and assists earthly in keeping builds self-contained, isolated, and repeatable.

### User Accounts
This tool utilizes software pulled from multiple sources and _some_ of them require authenticated access.  You will need to make an account at the following sites if you don't already have access:

- [Iron Bank](https://registry1.dso.mil/) : Platform One's authorized, hardened, and approved container repository. ([product](https://p1.dso.mil/#/products/iron-bank/) [pages](https://ironbank.dso.mil/))

  ---

&nbsp;

## Building

### Step 1 - Login to the Container Registry

This tool executes containerized builds within _secure containers_ so you'll need to be able to pull hardened images from Iron Bank.  Be sure you've logged Docker into the Iron Bank before attempting a build:

<table>
<tr valign="top">
<td>
<div>

```sh
docker login registry1.dso.mil -u <YOUR_USERNAME>
Password: <YOUR_CLI_SECRET>
```

</div>
<div>

---

**Harbor Login Credentials**

Iron Bank images are currently backed by an instance of the [Harbor](https://goharbor.io) registry.

To authenticate with Harbor via Docker you'll need to navigate to the Iron Bank [Harbor UI](https://registry1.dso.mil/harbor), login, and copy down your `CLI Secret`.

You should pass this `CLI Secret` **_instead of your password_** when invoking docker login!

---

</div>
</td>
<td width="503" height="415">
  <img src=".images/harbor-credentials.png">
</td>
</tr>
</table>

&nbsp;

### Step 2 - Run a Build

Building the package is one command:

```sh
make release
```

&nbsp;

### Step 3 - Test Drive

You can try out your new build with a local [Vagrant](https://www.vagrantup.com/) deployment, like so:

```bash
# To test RHEL 7 or 8
make test OS=rhel7
make test OS=rhel8

# To test ubuntu
make test OS=ubuntu

# escalate user once inside VM: vagrant --> root
sudo su
cd /opt/zarf
```

All OS options:
- rhel7
- rhel8
- centos7
- centos8
- ubuntu
- debian 

In less than a minute, you'll have a kubernetes cluster running all the pre-requisites needed to host and deploy mutliple other downstream clusters.

The status of the cluster creation can be monitored with `/usr/local/bin/k9s`

&nbsp;

### Step 4 - Cleanup

You can tear down the local [Vagrant](https://www.vagrantup.com/) deployment, like so:

```bash
# to deescalate user: root --> vagrant
exit

# to exit VM shell
exit

# tear down the VM
make test-close
```
