## Zarf Big Bang Single Package Example

This example demonstrates using Zarf in a very low-resources/singlue-use environment.  In this mode there is no gitops service and Zarf is simply a standard means of wrapping airgap concerns for K3s. This example deploys a basic K3s cluster using Traefik 2 and configures TLS / airgap concerns to deploy a single BB Package (twistlock).

### Steps to use:

1. `cd examples/`
2. Run one of these two commands:
   - `make all` - Download the latest version of Zarf, build the deploy package, and start a VM with Vagrant
   - `make all-dev` - Build Zarf locally, build the deploy package, and start a VM with Vagrant
3. Run: `./zarf init --confirm --components k3s` - Initialize Zarf, telling it to install k3s on your new VM. If you want to use interactive mode instead just run `./zarf init`.
4. Wait a bit, run `./zarf tools k9s` to see pods come up. Don't move on until everything is running
5. Run: `./zarf package deploy zarf-package-big-bang-core-demo.tar.zst --components kubescape --confirm` - Deploy Big Bang Core. If you want interactive mode instead just run `./zarf package deploy`, it will give you a picker to choose the package.
6. Wait several minutes. Run `./zarf tools k9s` to watch progress
8. Run `./zarf connect twistlock` to be taken to the twistlock consule in your browser.
9. When you're done, run `exit` to leave the VM then `make vm-destroy` to bring everything down
