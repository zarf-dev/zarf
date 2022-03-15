# Workstation Setup

There are several ways to use Zarf & the tooling needed depends on what you plan to do with it.  Here are some of the most common use cases, along with what you'll need to install on your workstation to play along.

&nbsp;

## Just gimmie Zarf!

The simplest path to Zarf is to download a pre-built release and execute it on your shell (just like any other CLI tool). To do that:

### Install

1. Point your browser at the current list of [Zarf releases](https://repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/-/releases).

1. Scroll to the version you want.

1. Download:

    - The zarf cluster initialization package: `zarf-init-<arch>.tar.zst`.

    - The appropriate zarf binary for your system (choose _one_):

        | system          | binary            |
        | ---             | ---               |
        | Linux (64bit)   | `zarf`            |
        | Intel-based Mac | `zarf-mac-intel`  |
        | [Apple-based Mac](https://support.apple.com/en-us/HT211814) | `zarf-mac-apple`  |

    - (optional) The checksum file: `zarf.sha256`.

1. (optional) Verify integrity of the downloaded files by validating their hashes&mdash;more about that ( [here](https://en.wikipedia.org/wiki/Checksum) / [here](https://help.ubuntu.com/community/HowToSHA256SUM) ) if you're interested. From _the directory holding your files_, run:

    ```sh
    shasum -c ./zarf.sha256

    > zarf: OK                  # <-- you should see "OK"s, like this
    > zarf-init-amd64.tar.zst: OK
    > zarf-init-arm64.tar.zst: OK
    > zarf-mac-apple: OK
    > zarf-mac-intel: OK
    ```

&nbsp;

### Try it out

Once you've got everything downloaded, you're ready to run commands directly against the zarf binary, like:

```sh
chmod +x ./zarf && ./zarf help

# substitute ./zarf-mac-intel or ./zarf-mac-apple above, as appropriate
```

&nbsp;

## I want a demo/example sandbox

If you're looking for an easy & low-risk way to evaluate Zarf, our recommendation is to pop into the `examples` folder.  Because the demos _aren't_ intended to be long-lived and _are_ expected to clean up after themselves they are a perfect way to kick the tires.

There are lots of ways to get a sandbox environment, here's two of them:

### Kubernetes-In-Docker (KinD)

1. Install [Docker](https://docs.docker.com/get-docker/). Other container engines will likely work as well but aren't actively tested by the Zarf team.

1. Install [KinD](https://github.com/kubernetes-sigs/kind). Other Kubernetes distros will work as well, but we'll be using KinD for this example since it is easy and tested frequently and thoroughly.

1. Run
   ```sh
   kind create cluster
   ```

That's it! You should now have a Kubernetes cluster running in Docker for use. Run `kind delete cluster` to clean up when you are done.

### Vagrant

You'll need to install _these_ tools to run the examples if you want to use Vagrant:

1. [Virtualbox ](https://www.virtualbox.org/wiki/Downloads) &mdash; The [hypervisor](https://www.redhat.com/en/topics/virtualization/what-is-a-hypervisor) we use to run our example VMs.

    > _**Take note**_
    >
    > We do _not_ use the VirtualBox Extension Pack as it is _not_ free for general use.  See ( [here](https://www.virtualbox.org/wiki/Licensing_FAQ) / [here](https://www.virtualbox.org/wiki/VirtualBox_PUEL) ) for details.

1. [Vagrant](https://www.vagrantup.com/downloads) &mdash; A CLI-based automation+workflow tool which greatly simplifies setup & use of VMs for development purposes.

1. [Make](https://www.gnu.org/software/make/) &mdash; The tool used to build / test the zarf binaries & record / execute general-purpose, project specific development tasks.

1. [Kustomize](https://kubectl.docs.kubernetes.io/installation/kustomize/binaries/) &mdash; Provides a template-free, patch-based mechanism for customizing Kubernetes configuration files.

    > _**Take note**_
    >
    > _Currently_ only used by the `big-bang` example but still required to start the singular example VM!

&nbsp;

### Try it out

Once you've got everything installed you're ready to run some examples! We recommend giving the [Get Started - game](../examples/game/README.md) example a try!
<!-- update link once Get Started page is written! -->

&nbsp;

## I need a dev machine

During dev & test, Zarf gets its exercise the same way the examples do&mdash;inside a VM.  Getting setup for development means that you'll need to install:

1. The [demo/example sandbox](#i-want-a-demoexample-sandbox) prerequisites &mdash; the virtualization stack we use for execution isolation.

1. [Go](https://golang.org/doc/install) &mdash; the programming language / build tools we use to create the `zarf` (et al.) binary.

    Currently required version is `1.16.x`.

&nbsp;

### Try it out

Once everything is installed, you're ready to build your _own_ version of Zarf. Give it a try using the instructions here: [Build Your First Zarf](./first-time-build.md).
