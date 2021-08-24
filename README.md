# Meet Zarf, K8s Airgap Buddy

Zarf is a static go binary that runs on various linux distros to deploy an airgap utility cluster including a docker registry and gitea server, batteries included. Zarf also included an [Appliance Mode](examples/appliance/README.md) that can be used for single-purpose k3s deployments.

> _This is a mirror of a government repo hosted on [Repo1](https://repo1.dso.mil/) by [DoD Platform One](http://p1.dso.mil/).  Please direct all code changes, issues and comments to https://repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf_

&nbsp;

![Zarf logo](.images/zarf-logo.png)

## Usage
General usage steps below.  For various ways to use Zarf, see [the examples folder](examples).  Please note that examples READMEs may replace the steps below.

### 1. Inital setup and config
- Download the files from the [Zarf Releases](https://repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/-/releases).
- (optional) Verify the downloads with `shasum -c zarf.sha256`.
- In a new folder or git repo, place a ZarfPackageConfig `zarf.yaml` with any changes you need to make, (see [the examples folder](examples) for more info).

  [![asciicast](https://asciinema.org/a/427846.svg)](https://asciinema.org/a/427846)
### 2. Create the zarf cluster
- Move the `zarf`, `zarf-init.tar.zst` files to the system you will install the cluster to. 
- Login or sudo/su to root.
- Run `./zarf init` and follow the wizard.
  [![asciicast](https://asciinema.org/a/427721.svg)](https://asciinema.org/a/427721)

### 3. Add resources to the zarf cluster 
- Folling step 1b, make any necessary edits to the `zarf.yaml` file.
- Then run `./zarf package create` to produce an `zarf-package-*.tar.zst` package.
- Move the `zarf-package` into the same folder on the running zarf cluster as in step 2a.
- Login or sudo/su to root.
- Run `./zarf package deploy` and follow the wizard.

  [![asciicast](https://asciinema.org/a/423449.svg)](https://asciinema.org/a/423449)

&nbsp;
## Development

## Prereqs

### User Accounts
This tool utilizes software pulled from multiple sources and _some_ of them require authenticated access.  You will need to make an account at the following sites if you don't already have access:

- [Iron Bank](https://registry1.dso.mil/) : Platform One's authorized, hardened, and approved container repository. ([product](https://p1.dso.mil/#/products/iron-bank/) [pages](https://ironbank.dso.mil/))

  ---

&nbsp;

## Building

### Step 1 - Login to the Container Registry

This tool executes containerized builds within _secure containers_ so you'll need to be able to pull hardened images from Iron Bank.  Be sure you've logged into the Iron Bank before attempting a build:

<table>
<tr valign="top">
<td>
<div>

```sh
zarf tools registry login registry1.dso.mil -u <YOUR_USERNAME>
Password: <YOUR_CLI_SECRET>
```

</div>
<div>

---

**Harbor Login Credentials**

Iron Bank images are currently backed by an instance of the [Harbor](https://goharbor.io) registry.

To authenticate with Harbor via zarf you'll need to navigate to the Iron Bank [Harbor UI](https://registry1.dso.mil/harbor), login, and copy down your `CLI Secret`.

You should pass this `CLI Secret` **_instead of your password_** when invoking zarf tools container login!

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
make build-test
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
- rocky

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
